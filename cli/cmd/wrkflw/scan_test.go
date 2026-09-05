package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScanFixture(t *testing.T, dir string, name string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func rulesOf(findings []finding) map[string]bool {
	rules := map[string]bool{}
	for _, item := range findings {
		rules[item.Rule] = true
	}
	return rules
}

func TestScanFindsSQLBuiltFromStrings(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "store.go", "package store\n\nfunc get(db *DB, name string) {\n\trow := db.QueryRow(\"SELECT id FROM users WHERE name = '\" + name + \"'\")\n\t_ = row\n}\n")
	writeScanFixture(t, dir, "repo.py", "def get(cursor, name):\n    cursor.execute(\"SELECT id FROM accounts WHERE name = '%s'\" % name)\n")
	writeScanFixture(t, dir, "repo.ts", "export function get(db: any, name: string) {\n  return db.query(`SELECT id FROM accounts WHERE name = '${name}'`)\n}\n")
	writeScanFixture(t, dir, "safe.go", "package store\n\nfunc get(db *DB, id string, userID string) {\n\trow := db.QueryRow(\"SELECT id FROM users WHERE id = $1 AND user_id = $2\", id, userID)\n\t_ = row\n}\n")
	writeScanFixture(t, dir, "sprintf.go", "package store\n\nimport \"fmt\"\n\nfunc lookup(db *DB, name string) {\n\trow := db.QueryRow(fmt.Sprintf(\"SELECT id FROM users WHERE name = '%s'\", name))\n\t_ = row\n}\n")

	findings, err := findingsInDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := rulesOf(findings)
	for _, rule := range []string{"sql-string-concat", "sql-string-format", "sql-template-literal"} {
		if !rules[rule] {
			t.Errorf("missing rule %q in %#v", rule, findings)
		}
	}
	sawSprintf := false
	for _, item := range findings {
		if item.File == "sprintf.go" && item.Rule == "sql-string-format" && item.Severity == severityHigh {
			sawSprintf = true
		}
	}
	if !sawSprintf {
		t.Errorf("Sprintf-built query not flagged high in %#v", findings)
	}
	for _, item := range findings {
		if item.File == "safe.go" {
			t.Errorf("placeholder query flagged: %#v", item)
		}
		if (item.Rule == "sql-string-format" || item.Rule == "sql-template-literal") && item.Severity != severityHigh {
			t.Errorf("SQL rule severity = %q, want high", item.Severity)
		}
		if item.Rule == "sql-string-concat" && item.Severity != severityMedium {
			t.Errorf("concat rule severity = %q, want medium for human review", item.Severity)
		}
	}
}

func TestScanFindsSecretsCommandInjectionAndXSS(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "config.py", "AWS_KEY = \"AKIAIOSFODNN7EXAMPLE\"\n")
	writeScanFixture(t, dir, "deploy.sh", "eval \"run $PLAN\"\n")
	writeScanFixture(t, dir, "run.go", "package main\n\nimport \"os/exec\"\n\nfunc deploy(plan string) {\n\tcmd := exec.Command(\"sh\", \"-c\", \"run \"+plan)\n\t_ = cmd\n}\n")
	writeScanFixture(t, dir, "view.tsx", "export function Bio({bio}: {bio: string}) {\n  return <div dangerouslySetInnerHTML={{__html: bio}} />\n}\n")
	writeScanFixture(t, dir, "notes.md", "AKIAIOSFODNN7EXAMPLE lives here but markdown is out of scope\n")

	findings, err := findingsInDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := rulesOf(findings)
	for _, rule := range []string{"hardcoded-secret", "shell-interpolation", "unescaped-html-sink"} {
		if !rules[rule] {
			t.Errorf("missing rule %q in %#v", rule, findings)
		}
	}
	for _, item := range findings {
		if item.File == "notes.md" {
			t.Errorf("out-of-scope extension scanned: %#v", item)
		}
		if item.BlastRadius == "" || item.Fingerprint == "" {
			t.Errorf("finding lacks blast radius or fingerprint: %#v", item)
		}
	}
}

func TestScanSkipsVendoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "node_modules/evil.js", "db.query(`SELECT * FROM t WHERE a = '${x}'`)\n")
	writeScanFixture(t, dir, "ok.js", "db.query(`SELECT * FROM t WHERE a = '${x}'`)\n")

	findings, err := findingsInDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("ok.js produced no findings")
	}
	for _, item := range findings {
		if item.File != "ok.js" {
			t.Fatalf("vendored file scanned: %#v", item)
		}
	}
}

func TestScanFingerprintsAreStable(t *testing.T) {
	first := fingerprint("sql-string-concat", "store.go", "db.QueryRow(\"SELECT\" + name)")
	second := fingerprint("sql-string-concat", "store.go", "  db.QueryRow(\"SELECT\" + name)  ")
	other := fingerprint("sql-string-concat", "store.go", "db.QueryRow(\"SELECT\" + id)")
	if first != second {
		t.Fatal("same finding hashed differently after whitespace drift")
	}
	if first == other {
		t.Fatal("different findings share a fingerprint")
	}
	if len(first) != 16 {
		t.Fatalf("fingerprint %q is not 16 hex chars", first)
	}
}

func TestScanSeverityFilterAndExit(t *testing.T) {
	findings := []finding{
		{Rule: "a", Severity: severityLow},
		{Rule: "b", Severity: severityMedium},
		{Rule: "c", Severity: severityHigh},
	}
	if got := filterFindings(findings, severityMedium); len(got) != 2 {
		t.Fatalf("medium filter kept %d, want 2", len(got))
	}
	if got := filterFindings(findings, severityHigh); len(got) != 1 || got[0].Rule != "c" {
		t.Fatalf("high filter = %#v", got)
	}
	var err error = scanFindingsError{high: 1, total: 3}
	if err == nil || !strings.Contains(err.Error(), "3 finding(s)") {
		t.Fatalf("exit error = %v", err)
	}
}

func TestScanTaskBodiesAreActionable(t *testing.T) {
	item := finding{Rule: "sql-string-concat", Severity: severityHigh, File: "store.go", Line: 12, Message: "use placeholders", BlastRadius: "account", Fingerprint: "abc123"}
	if title := scanTaskTitle(item); !strings.Contains(title, "sql-string-concat") || !strings.Contains(title, "store.go:12") {
		t.Fatalf("title = %q", title)
	}
	body := scanTaskBody(item)
	for _, want := range []string{"account", "abc123", "review"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q lacks %q", body, want)
		}
	}
}

func TestScanRejectsBadInput(t *testing.T) {
	if _, err := findingsInDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing directory succeeded")
	}
	if _, err := parseSeverity("critical"); err == nil {
		t.Fatal("bad severity succeeded")
	}
}
