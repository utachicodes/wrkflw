package main

// wrkflw scan statically screens a codebase for the vulnerability shapes
// that most often turn into incidents: SQL built from strings, hardcoded
// secrets, unscoped data access, shell execution of interpolated text, and
// unescaped HTML sinks.
//
// The differentiator is the fix loop, not the detectors. Every finding
// carries a stable fingerprint, and --create-tasks files each finding as a
// wrkflw inbox task under an idempotency key derived from that fingerprint.
// Re-running the scan never duplicates tasks. Assign the tasks to a security
// agent, and each fix travels the normal path: claim, isolated worktree,
// output, human review. Findings also carry a blast radius: the narrowest
// credential scope that could exploit them, mapped to wrkflw's trust model.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Severity ranks a finding. High fails the command.
type severity string

const (
	severityLow    severity = "low"
	severityMedium severity = "medium"
	severityHigh   severity = "high"
)

func parseSeverity(value string) (severity, error) {
	switch severity(strings.ToLower(strings.TrimSpace(value))) {
	case severityLow:
		return severityLow, nil
	case severityMedium:
		return severityMedium, nil
	case severityHigh:
		return severityHigh, nil
	default:
		return "", fmt.Errorf("invalid severity %q; choose low, medium, or high", value)
	}
}

func (s severity) atLeast(min severity) bool {
	rank := map[severity]int{severityLow: 0, severityMedium: 1, severityHigh: 2}
	return rank[s] >= rank[min]
}

// finding is one matched vulnerability shape.
type finding struct {
	Rule        string   `json:"rule"`
	Severity    severity `json:"severity"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Message     string   `json:"message"`
	BlastRadius string   `json:"blastRadius"`
	Fingerprint string   `json:"fingerprint"`
}

type detector struct {
	rule        string
	severity    severity
	extensions  map[string]bool
	match       *regexp.Regexp
	exclude     *regexp.Regexp
	message     string
	blastRadius string
}

var scanDetectors = []detector{
	{
		rule:       "sql-string-format",
		severity:   severityHigh,
		extensions: map[string]bool{".go": true},
		match:      regexp.MustCompile(`\.(Query|QueryRow|Exec)(Context)?\s*\(\s*(ctx\s*,\s*)?fmt\.Sprintf`),
		message:    "SQL text built with Sprintf flows into a query; use placeholders ($1, ?) instead",
		blastRadius: "account: any input reaching this query can read or write other rows",
	},
	{
		rule:       "sql-string-concat",
		severity:   severityMedium,
		extensions: map[string]bool{".go": true},
		match:      regexp.MustCompile(`\.(Query|QueryRow|Exec)(Context)?\s*\([^)]*\+`),
		exclude:    regexp.MustCompile(`fmt\.Sprintf`),
		message:    "string concatenation inside a query call; confirm every part is a constant fragment or placeholder, not input",
		blastRadius: "account: confirm no input reaches this query outside placeholders",
	},
	{
		rule:       "sql-string-format",
		severity:   severityHigh,
		extensions: map[string]bool{".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true},
		match:      regexp.MustCompile(`(?i)(execute|query|raw)\s*\([^)]*(\.format\(|["']\s*%|%\s*[A-Za-z_(\[]|\+|__format__|f["'])`),
		exclude:    regexp.MustCompile(`^\s*(//|#|\*|SELECT|INSERT|UPDATE|DELETE)`),
		message:    "SQL built with formatting or concatenation flows into execute; use parameter binding instead",
		blastRadius: "account: any input reaching this query can read or write other rows",
	},
	{
		rule:       "sql-template-literal",
		severity:   severityHigh,
		extensions: map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true},
		match:      regexp.MustCompile("(query|execute|raw)\\s*\\(\\s*`[^`]*\\$\\{"),
		message:    "template literal interpolation inside a query; use parameter binding instead",
		blastRadius: "account: any input reaching this query can read or write other rows",
	},
	{
		rule:       "unscoped-data-access",
		severity:   severityMedium,
		extensions: map[string]bool{".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true},
		match:      regexp.MustCompile(`(?i)\b(SELECT|UPDATE|DELETE)\b.*\b(FROM|INTO)\b\s+\w+`),
		exclude:    regexp.MustCompile(`(?i)(user_id|account_id|owner|tenant|schema_migrations|information_schema|pg_)`),
		message:    "data access with no visible account scoping; confirm the query constrains rows to the authenticated owner",
		blastRadius: "account: a missing owner check exposes other accounts' rows",
	},
	{
		rule:       "hardcoded-secret",
		severity:   severityHigh,
		extensions: map[string]bool{".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".sh": true, ".env": true, ".yaml": true, ".yml": true, ".toml": true, ".json": true},
		match:      regexp.MustCompile(`(AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[bap]-[A-Za-z0-9-]{10,}|-----BEGIN [A-Z ]*PRIVATE KEY-----)`),
		message:    "recognizable live credential committed to the tree; revoke it and move it to the environment",
		blastRadius: "account: whoever holds this secret inherits its full scope",
	},
	{
		rule:       "assigned-secret",
		severity:   severityMedium,
		extensions: map[string]bool{".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true},
		match:      regexp.MustCompile(`(?i)(password|passwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*["'][^"' ]{4,}["']`),
		exclude:    regexp.MustCompile(`(?i)(test|example|placeholder|changeme|xxx|hash|placeholder)`),
		message:    "literal secret assigned in code; confirm it is a test fixture or move it to the environment",
		blastRadius: "account: a real secret here inherits its full scope if leaked",
	},
	{
		rule:       "shell-interpolation",
		severity:   severityHigh,
		extensions: map[string]bool{".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".sh": true},
		match:      regexp.MustCompile("(os\\.system\\s*\\(|shell\\s*=\\s*True|exec\\.Command\\s*\\([^)]*(fmt\\.Sprintf|\\+)|child_process\\.(exec|execSync)\\s*\\([^)]*(\\+|\\$\\{|%s)|\\beval\\s*\\(`|\\beval\\s+[\"'])"),
		message:    "interpolated text reaches a shell; pass arguments as a list instead",
		blastRadius: "machine: shell injection runs with the process credentials",
	},
	{
		rule:       "unescaped-html-sink",
		severity:   severityMedium,
		extensions: map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true},
		match:      regexp.MustCompile(`(dangerouslySetInnerHTML|\.innerHTML\s*=[^=][^;]*(\+|\$\{))`),
		message:    "dynamic content reaches an HTML sink; escape or sanitize it first",
		blastRadius: "session: stored markup runs in another user's browser",
	},
}

var scanSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, "build": true, "__pycache__": true, ".venv": true,
	"venv": true, ".next": true, "coverage": true,
}

const maxScanFileBytes = 2 << 20

func scanExtensions() map[string]bool {
	extensions := map[string]bool{}
	for _, detector := range scanDetectors {
		for extension := range detector.extensions {
			extensions[extension] = true
		}
	}
	return extensions
}

// findingsInDir walks root and returns every matched finding, sorted for
// stable output. Paths in findings are slash-separated and root-relative.
func findingsInDir(root string) ([]finding, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan target %q is not a directory", root)
	}
	extensions := scanExtensions()
	var findings []finding
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && scanSkipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !extensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		if info, err := entry.Info(); err != nil || info.Size() > maxScanFileBytes {
			return nil
		}
		findings = append(findings, scanFile(root, path)...)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Rule < findings[j].Rule
	})
	return findings, nil
}

func scanFile(root string, path string) []finding {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		relative = path
	}
	relative = filepath.ToSlash(relative)
	extension := strings.ToLower(filepath.Ext(path))
	var findings []finding
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, detector := range scanDetectors {
			if !detector.extensions[extension] {
				continue
			}
			if !detector.match.MatchString(line) {
				continue
			}
			if detector.exclude != nil && detector.exclude.MatchString(line) {
				continue
			}
			findings = append(findings, finding{
				Rule:        detector.rule,
				Severity:    detector.severity,
				File:        relative,
				Line:        lineNumber + 1,
				Message:     detector.message,
				BlastRadius: detector.blastRadius,
				Fingerprint: fingerprint(detector.rule, relative, trimmed),
			})
		}
	}
	return findings
}

// fingerprint is stable across re-runs: it hashes what was found, not where
// the line number drifted to, so --create-tasks never files duplicates.
func fingerprint(rule string, file string, line string) string {
	sum := sha256.Sum256([]byte(rule + "\x00" + file + "\x00" + strings.TrimSpace(line)))
	return hex.EncodeToString(sum[:])[:16]
}

func filterFindings(findings []finding, minimum severity) []finding {
	var kept []finding
	for _, item := range findings {
		if item.Severity.atLeast(minimum) {
			kept = append(kept, item)
		}
	}
	if kept == nil {
		return []finding{}
	}
	return kept
}

func countBySeverity(findings []finding) map[string]int {
	counts := map[string]int{"low": 0, "medium": 0, "high": 0}
	for _, item := range findings {
		counts[string(item.Severity)]++
	}
	return counts
}

func scanTaskTitle(item finding) string {
	return fmt.Sprintf("[scan:%s] %s:%d", item.Rule, item.File, item.Line)
}

func scanTaskBody(item finding) string {
	return fmt.Sprintf("Severity: %s\nLocation: %s:%d\nBlast radius: %s\nFingerprint: %s\n\n%s\n\nFix the code, then have a person review the change before it ships.",
		item.Severity, item.File, item.Line, item.BlastRadius, item.Fingerprint, item.Message)
}

func scanCmd(c client, args []string) error {
	if wantsHelp(args) {
		return printHelp("scan")
	}
	fs := newFlagSet("scan")
	minSeverityFlag := fs.String("min-severity", "low", "lowest severity to report: low, medium, or high")
	createTasks := fs.Bool("create-tasks", false, "file each finding as an inbox task (WRKFLW_API_TOKEN required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	minimum, err := parseSeverity(*minSeverityFlag)
	if err != nil {
		return err
	}
	root := "."
	if fs.NArg() > 1 {
		return errors.New("usage: wrkflw scan [dir] [--min-severity low|medium|high] [--create-tasks]")
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}
	all, err := findingsInDir(root)
	if err != nil {
		return err
	}
	matched := filterFindings(all, minimum)
	created := 0
	if *createTasks {
		for _, item := range matched {
			body := map[string]any{"title": scanTaskTitle(item), "description": scanTaskBody(item)}
			var createdTask struct {
				ID string `json:"id"`
			}
			headers := map[string]string{"Idempotency-Key": "scan-" + item.Fingerprint}
			if err := c.doWithHeaders(http.MethodPost, "/api/v1/tasks", body, headers, &createdTask); err != nil {
				return err
			}
			created++
		}
	}
	report := map[string]any{
		"root":     root,
		"findings": matched,
		"counts":   countBySeverity(matched),
		"created":  created,
	}
	if err := printJSON(report); err != nil {
		return err
	}
	if len(matched) > 0 {
		return scanFindingsError{high: countBySeverity(matched)["high"], total: len(matched)}
	}
	return nil
}

// scanFindingsError fails the command when findings remain. The message
// stays actionable: rerun with --min-severity high to gate releases.
type scanFindingsError struct {
	high  int
	total int
}

func (e scanFindingsError) Error() string {
	return fmt.Sprintf("scan found %d finding(s), %d high; fix them or rerun with --min-severity high to gate on highs only", e.total, e.high)
}
