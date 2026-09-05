package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://wrkflw"

const maxTaskEntryBodyBytes = 16 * 1024

var version = "dev"

type client struct {
	baseURL string
	token   string
	http    *http.Client
	stdin   io.Reader
	runID   string
	// ctx cancels in-flight requests. The watcher sets it so an interrupt does
	// not have to wait for the client timeout.
	ctx context.Context
}

type APIError struct {
	Status             int
	StatusCode         int
	Code               string
	Message            string
	Body               string
	RetryAfter         string
	RetryAfterDuration time.Duration
}

func (e *APIError) Error() string {
	status := e.StatusCode
	if status == 0 {
		status = e.Status
	}
	detail := e.Body
	if detail == "" {
		detail = e.Message
	}
	return fmt.Sprintf("wrkflw API %d: %s", status, detail)
}

// withContext returns a copy whose requests are cancelled with ctx.
func (c client) withContext(ctx context.Context) client {
	c.ctx = ctx
	return c
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return printHelp("")
	}
	c := client{
		baseURL: env("WRKFLW_BASE_URL", defaultBaseURL),
		token:   os.Getenv("WRKFLW_API_TOKEN"),
		runID:   env("WRKFLW_RUN_ID", ""),
		http:    &http.Client{Timeout: 30 * time.Second},
		stdin:   os.Stdin,
	}
	switch args[1] {
	case "help", "-h", "--help":
		topic := ""
		if len(args) > 2 {
			topic = args[2]
		}
		return printHelp(topic)
	case "version", "--version":
		return printVersion(args[2:], os.Stdout)
	case "auth":
		return authCmd(c, args[2:])
	case "lists", "buckets":
		return listsCmd(c, args[2:])
	case "tasks":
		return tasksCmd(c, args[2:])
	case "watch":
		return watchCmd(c, args[2:])
	case "runs":
		return runsCmd(args[2:])
	case "scan":
		return scanCmd(c, args[2:])
	default:
		return fmt.Errorf("unknown command %q; run 'wrkflw help'", args[1])
	}
}

func printVersion(args []string, w io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: wrkflw version")
	}
	return json.NewEncoder(w).Encode(map[string]string{"version": version})
}

func printHelp(topic string) error {
	help, ok := helpText[topic]
	if !ok {
		return fmt.Errorf("unknown help topic %q; choose auth, lists, tasks, watch, runs, or scan", topic)
	}
	_, err := fmt.Fprint(os.Stdout, help)
	return err
}

var helpText = map[string]string{
	"": `Wrkflw CLI controls lists, tasks, and agent workflow.

Configuration:
  WRKFLW_API_TOKEN   Required API token created in Wrkflw settings
  WRKFLW_BASE_URL    API URL (default: https://wrkflw)
  WRKFLW_RUN_ID      Managed execution run ID supplied by the watcher

Usage:
  wrkflw version
  wrkflw help [auth|lists|tasks|watch|runs|scan]
  wrkflw auth status
  wrkflw lists <command>
  wrkflw tasks <command>
  wrkflw watch --profile <name>
  wrkflw runs <command>
  wrkflw scan [dir]

All successful command output is JSON. IDs are returned by list/get commands.
Run "wrkflw help <topic>" for every command and flag.
`,
	"watch": `Usage:
  wrkflw watch --profile <name> [--list <list-id>] [--workdir <git-path>]

Runs one configured agent against its assigned Ready tasks. The watcher pins
the source commit at startup. For each task it creates a disposable Git
worktree from that commit, starts the profile's executor there with the task
prompt on stdin, and watches that exact run. Restart after updating the source
branch. The agent claims the task, does the work, and reports through the Wrkflw
CLI. The watcher never claims or writes to the task itself.

The source checkout must be on a named branch with nothing uncommitted, and
--workdir defaults to the current directory. An executor never runs in it.

--list limits both the search for work and the check for a task already in
progress. A task already in progress stops startup: finish it or move it back
to Ready, because this version has no automatic resume.

Successful, blocked, interrupted, and uncertain worktrees are kept for
inspection. Runs that lose the claim or exit before ever claiming are deleted.
A profile keeps at most 10 retained worktrees; at the limit no new run starts
until one is released.

Profiles live in WRKFLW_CONFIG, or wrkflw/config.json under the user
configuration directory:

  {
    "profiles": {
      "codex": {
        "agentId": "<the agent's Wrkflw ID>",
        "tokenEnv": "WRKFLW_CODEX_TOKEN",
        "command": ["codex", "exec", "-"]
      }
    }
  }

The token itself is never stored, only the name of the variable holding it.
Profile changes take effect when the watcher restarts.
`,
	"runs": `Usage:
  wrkflw runs list [--profile <name>]
  wrkflw runs clean <run-id>

"list" shows retained runs and where their worktrees are. "clean" releases one
worktree once nothing from the run is running and the worktree is clean. It
never forces and it keeps the branch, so any commits stay reachable.
`,
	"scan": `Usage:
  wrkflw scan [dir] [--min-severity low|medium|high] [--create-tasks]

Screens a codebase for SQL built from strings, hardcoded secrets, unscoped
data access, shell execution of interpolated text, and unescaped HTML sinks.
Exits non-zero when findings at or above --min-severity remain, so it gates
releases in CI.

--create-tasks files each finding as an inbox task under a stable idempotency
key derived from the finding, so re-runs never duplicate tasks. Assign the
tasks to a security agent: each fix travels the normal path of claim,
isolated worktree, output, and human review. Needs WRKFLW_API_TOKEN.
`,
	"auth": `Usage:
  wrkflw auth status                 Verify the token and show its user
`,
	"lists": `Usage:
  wrkflw lists list
  wrkflw lists get <list-id>
  wrkflw lists create --name <name> [--goal <goal>] [--inbox]
  wrkflw lists update <list-id> [--name <name>] [--goal <goal>] [--inbox=true|false] [--sort-order <n>]
  wrkflw lists delete <list-id>
  wrkflw lists reorder <list-id>...

"get" returns every active item and the 20 most recently updated completed
items in the list. Use "tasks list --status done" to page older completed work.

"buckets" is accepted as an alias for "lists".
`,
	"tasks": `Usage:
  wrkflw tasks list [--list <list-id>] [--status <status>] [--priority <p0|p1|p2|p3>] [--limit <n>] [--cursor <cursor>]
  wrkflw tasks get <task-id>
  wrkflw tasks pull [--list <list-id>] [--priority <p0|p1|p2|p3>] [--limit <n>]
  wrkflw tasks create --title <title> [--list <list-id> | --parent <task-id>] [--description <text>] [--date <YYYY-MM-DD>] [--idempotency-key <key>]
  wrkflw tasks update <task-id> [--title <title>] [--description <text>] [--date <YYYY-MM-DD>] [--list <list-id>] [--priority <p0|p1|p2|p3>]
  wrkflw tasks delete <task-id>
  wrkflw tasks reorder --list <list-id> <task-id>...
  wrkflw tasks claim <task-id>
  wrkflw tasks entries <task-id> [--run <run-id>]
  wrkflw tasks comment <task-id> (--body <text> | --file <path>) [--idempotency-key <key>]
  wrkflw tasks output <task-id> (--body <text> | --file <path>) --idempotency-key <key>
  wrkflw tasks status <task-id> new|queued|working|needs_review|done

"pull" returns open queued tasks. Claim before starting work. Use an empty
--description or --date value to clear that field, or an empty --priority to
clear the priority. "working" uses the atomic claim operation, so only one
agent can successfully claim a queued task.
Reuse --idempotency-key when retrying task creation after an uncertain result.
Use --file - to read a comment or output from stdin. Managed comments require
an idempotency key. Reuse the same key when retrying a comment or output.
Task collections omit descriptions. Use "tasks get" for complete task detail.
Completed pages default to 20 items and return nextCursor for --cursor.
`,
}

func authCmd(c client, args []string) error {
	if wantsHelp(args) {
		return printHelp("auth")
	}
	if len(args) != 1 || args[0] != "status" {
		return errors.New("usage: wrkflw auth status; run 'wrkflw help auth'")
	}
	return c.getJSON("/api/v1/me", nil)
}

func listsCmd(c client, args []string) error {
	if wantsHelp(args) {
		return printHelp("lists")
	}
	if len(args) < 1 {
		return errors.New("usage: wrkflw lists <command>; run 'wrkflw help lists'")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: wrkflw lists list")
		}
		return c.getJSON("/api/v1/lists", nil)
	case "get":
		id, err := singleID("wrkflw lists get <list-id>", args[1:])
		if err != nil {
			return err
		}
		return c.getJSON("/api/v1/lists/"+url.PathEscape(id), nil)
	case "create":
		fs := newFlagSet("lists create")
		name := fs.String("name", "", "list name")
		goal := fs.String("goal", "", "list goal")
		inbox := fs.Bool("inbox", false, "make this the inbox")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || strings.TrimSpace(*name) == "" {
			return errors.New("--name is required")
		}
		body := map[string]any{"name": *name, "goal": *goal, "isInbox": *inbox}
		return c.sendJSON(http.MethodPost, "/api/v1/lists", body)
	case "update":
		if len(args) < 2 {
			return errors.New("usage: wrkflw lists update <list-id> [flags]")
		}
		id := args[1]
		fs := newFlagSet("lists update")
		name := fs.String("name", "", "list name")
		goal := fs.String("goal", "", "list goal")
		inbox := fs.Bool("inbox", false, "set or clear inbox status")
		sortOrder := fs.Int("sort-order", 0, "sort order")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		body := visitedValues(fs, map[string]any{"name": *name, "goal": *goal, "inbox": *inbox, "sort-order": *sortOrder}, map[string]string{"inbox": "isInbox", "sort-order": "sortOrder"})
		if len(body) == 0 {
			return errors.New("at least one update flag is required")
		}
		return c.sendJSON(http.MethodPatch, "/api/v1/lists/"+url.PathEscape(id), body)
	case "delete":
		id, err := singleID("wrkflw lists delete <list-id>", args[1:])
		if err != nil {
			return err
		}
		return c.sendJSON(http.MethodDelete, "/api/v1/lists/"+url.PathEscape(id), nil)
	case "reorder":
		fs := newFlagSet("lists reorder")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			return errors.New("at least one list id is required")
		}
		return c.sendJSON(http.MethodPost, "/api/v1/lists/reorder", map[string]any{"ids": fs.Args()})
	default:
		return fmt.Errorf("unknown lists command %q; run 'wrkflw help lists'", args[0])
	}
}

func tasksCmd(c client, args []string) error {
	if wantsHelp(args) {
		return printHelp("tasks")
	}
	if len(args) < 1 {
		return errors.New("usage: wrkflw tasks <command>; run 'wrkflw help tasks'")
	}
	switch args[0] {
	case "list", "pull":
		command := args[0]
		fs := newFlagSet("tasks " + command)
		listID := fs.String("list", "", "list id")
		limit := fs.Int("limit", 0, "maximum tasks")
		priority := fs.String("priority", "", "priority filter: p0, p1, p2, or p3")
		var status, cursor *string
		if command == "list" {
			status = fs.String("status", "", "status filter")
			cursor = fs.String("cursor", "", "completed history cursor")
		}
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		if !validPriority(*priority) {
			return fmt.Errorf("invalid priority %q; choose p0, p1, p2, or p3", *priority)
		}
		q := url.Values{}
		setQuery(q, "bucketId", *listID)
		setQuery(q, "priority", *priority)
		if *limit > 0 {
			q.Set("limit", strconv.Itoa(*limit))
		}
		if status != nil {
			setQuery(q, "status", *status)
			setQuery(q, "cursor", *cursor)
		}
		path := "/api/v1/tasks"
		if command == "pull" {
			path = "/api/v1/agent/tasks"
		}
		return c.getJSON(path, q)
	case "get":
		id, err := singleID("wrkflw tasks get <task-id>", args[1:])
		if err != nil {
			return err
		}
		return c.getJSON("/api/v1/tasks/"+url.PathEscape(id), nil)
	case "create":
		fs := newFlagSet("tasks create")
		listID := fs.String("list", "", "list id")
		bucketID := fs.String("bucket", "", "deprecated alias for --list")
		parentID := fs.String("parent", "", "parent task id for a subtask")
		title := fs.String("title", "", "task title")
		description := fs.String("description", "", "task description")
		date := fs.String("date", "", "planned date")
		idempotencyKey := fs.String("idempotency-key", "", "stable key for safe retries")
		override := fs.Bool("override-limit", false, "deprecated compatibility flag; Lists no longer reject tasks by count")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		targetList := firstNonEmpty(*listID, *bucketID)
		if fs.NArg() != 0 || strings.TrimSpace(*title) == "" {
			return errors.New("--title is required")
		}
		if targetList != "" && strings.TrimSpace(*parentID) != "" {
			return errors.New("choose --list or --parent, not both")
		}
		body := map[string]any{"title": *title, "description": *description, "scheduledDate": *date, "kind": "action", "overrideLimit": *override}
		path := "/api/v1/tasks"
		if targetList != "" {
			path = "/api/v1/lists/" + url.PathEscape(targetList) + "/tasks"
		} else if strings.TrimSpace(*parentID) != "" {
			path = "/api/v1/tasks/" + url.PathEscape(strings.TrimSpace(*parentID)) + "/subtasks"
		}
		return c.sendJSONWithHeaders(http.MethodPost, path, body, map[string]string{"Idempotency-Key": *idempotencyKey})
	case "update":
		if len(args) < 2 {
			return errors.New("usage: wrkflw tasks update <task-id> [flags]")
		}
		id := args[1]
		fs := newFlagSet("tasks update")
		title := fs.String("title", "", "title")
		description := fs.String("description", "", "description")
		date := fs.String("date", "", "planned date")
		listID := fs.String("list", "", "list id")
		bucketID := fs.String("bucket", "", "deprecated alias for --list")
		priority := fs.String("priority", "", "priority: p0, p1, p2, p3, or empty to clear")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		if !validPriority(*priority) {
			return fmt.Errorf("invalid priority %q; choose p0, p1, p2, p3, or an empty value to clear", *priority)
		}
		body := map[string]any{}
		fs.Visit(func(item *flag.Flag) {
			switch item.Name {
			case "title":
				body["title"] = *title
			case "description":
				body["description"] = *description
			case "date":
				body["scheduledDate"] = *date
			case "priority":
				body["priority"] = *priority
			}
		})
		if targetList := firstNonEmpty(*listID, *bucketID); targetList != "" {
			body["bucketId"] = targetList
		}
		if len(body) == 0 {
			return errors.New("at least one update flag is required")
		}
		return c.sendJSONWithHeaders(http.MethodPatch, "/api/v1/tasks/"+url.PathEscape(id), body, managedRunHeaders())
	case "delete":
		id, err := singleID("wrkflw tasks delete <task-id>", args[1:])
		if err != nil {
			return err
		}
		return c.sendJSON(http.MethodDelete, "/api/v1/tasks/"+url.PathEscape(id), nil)
	case "reorder":
		fs := newFlagSet("tasks reorder")
		listID := fs.String("list", "", "list id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *listID == "" || fs.NArg() == 0 {
			return errors.New("--list and at least one task id are required")
		}
		return c.sendJSON(http.MethodPost, "/api/v1/lists/"+url.PathEscape(*listID)+"/reorder-tasks", map[string]any{"ids": fs.Args()})
	case "claim":
		id, err := singleID("wrkflw tasks claim <task-id>", args[1:])
		if err != nil {
			return err
		}
		return c.sendJSONWithHeaders(http.MethodPost, "/api/v1/agent/tasks/"+url.PathEscape(id)+"/claim", map[string]any{}, managedRunHeaders())
	case "entries":
		if len(args) < 2 {
			return errors.New("usage: wrkflw tasks entries <task-id> [--run <run-id>]")
		}
		id := args[1]
		fs := newFlagSet("tasks entries")
		runID := fs.String("run", "", "exact managed run id")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || strings.TrimSpace(id) == "" {
			return errors.New("usage: wrkflw tasks entries <task-id> [--run <run-id>]")
		}
		q := url.Values{}
		setQuery(q, "runId", *runID)
		return c.getJSON("/api/v1/tasks/"+url.PathEscape(id)+"/entries", q)
	case "comment", "output":
		return taskEntryCmd(c, args[0], args[1:])
	case "status":
		if len(args) != 3 {
			return errors.New("usage: wrkflw tasks status <task-id> new|queued|working|needs_review|done")
		}
		if !validStatus(args[2]) {
			return fmt.Errorf("invalid status %q; choose new, queued, working, needs_review, or done", args[2])
		}
		if args[2] == "working" && env("WRKFLW_RUN_ID", "") == "" {
			return c.sendJSONWithHeaders(http.MethodPost, "/api/v1/agent/tasks/"+url.PathEscape(args[1])+"/claim", map[string]any{}, managedRunHeaders())
		}
		return c.sendJSONWithHeaders(http.MethodPatch, "/api/v1/agent/tasks/"+url.PathEscape(args[1])+"/status", map[string]any{"status": args[2]}, managedRunHeaders())
	default:
		return fmt.Errorf("unknown tasks command %q; run 'wrkflw help tasks'", args[0])
	}
}

func taskEntryCmd(c client, kind string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: wrkflw tasks %s <task-id> (--body <text> | --file <path>) --idempotency-key <key>", kind)
	}
	id := args[0]
	fs := newFlagSet("tasks " + kind)
	bodyValue := fs.String("body", "", "entry body")
	fileValue := fs.String("file", "", "read entry body from a file, or - for stdin")
	idempotencyKey := fs.String("idempotency-key", "", "stable key for safe retries")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(id) == "" {
		return errors.New("unexpected arguments")
	}
	bodySet := false
	fileSet := false
	fs.Visit(func(item *flag.Flag) {
		switch item.Name {
		case "body":
			bodySet = true
		case "file":
			fileSet = true
		}
	})
	if bodySet == fileSet {
		return errors.New("exactly one of --body or --file is required")
	}
	if kind == "output" && strings.TrimSpace(*idempotencyKey) == "" {
		return errors.New("--idempotency-key is required for output")
	}
	if env("WRKFLW_RUN_ID", "") != "" && strings.TrimSpace(*idempotencyKey) == "" {
		return errors.New("--idempotency-key is required for managed comments and outputs")
	}
	body := []byte(*bodyValue)
	if fileSet {
		var err error
		body, err = c.readTaskEntryBody(*fileValue)
		if err != nil {
			return err
		}
	}
	if len(body) > maxTaskEntryBodyBytes {
		return fmt.Errorf("entry body is %d bytes; limit is %d bytes", len(body), maxTaskEntryBodyBytes)
	}
	if strings.TrimSpace(string(body)) == "" {
		return errors.New("entry body is required")
	}
	headers := managedRunHeaders()
	headers["Idempotency-Key"] = strings.TrimSpace(*idempotencyKey)
	return c.sendJSONWithHeaders(http.MethodPost, "/api/v1/tasks/"+url.PathEscape(id)+"/entries", map[string]any{
		"kind": kind,
		"body": string(body),
	}, headers)
}

func (c client) readTaskEntryBody(path string) ([]byte, error) {
	if path == "-" {
		reader := c.stdin
		if reader == nil {
			reader = os.Stdin
		}
		return readBoundedTaskEntryBody(reader)
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("--file requires a path or - for stdin")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readBoundedTaskEntryBody(file)
}

func readBoundedTaskEntryBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxTaskEntryBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxTaskEntryBodyBytes {
		return nil, fmt.Errorf("entry body exceeds %d bytes", maxTaskEntryBodyBytes)
	}
	return body, nil
}

func managedRunHeaders() map[string]string {
	return map[string]string{"X-Wrkflw-Run-ID": env("WRKFLW_RUN_ID", "")}
}

func (c client) getJSON(path string, q url.Values) error {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out any
	if err := c.do(http.MethodGet, path, nil, &out); err != nil {
		return err
	}
	return printJSON(out)
}

func (c client) sendJSON(method string, path string, body any) error {
	return c.sendJSONWithHeaders(method, path, body, nil)
}

func (c client) sendJSONWithHeaders(method string, path string, body any, headers map[string]string) error {
	var out any
	if err := c.doWithHeaders(method, path, body, headers, &out); err != nil {
		return err
	}
	return printJSON(out)
}

func (c client) do(method string, path string, body any, target any) error {
	return c.doWithHeaders(method, path, body, nil, target)
}

func (c client) doWithHeaders(method string, path string, body any, headers map[string]string, target any) error {
	if c.token == "" {
		return errors.New("WRKFLW_API_TOKEN is required; create one in Wrkflw settings")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	requestCtx := c.ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	req, err := http.NewRequestWithContext(requestCtx, method, strings.TrimRight(c.baseURL, "/")+path, reader)
	if err != nil {
		return &requestBuildError{err: err}
	}
	if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.Host == "" {
		return &requestBuildError{err: fmt.Errorf("WRKFLW_BASE_URL must be an absolute http or https URL")}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	for name, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(name, value)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body := safeAPIErrorBody(raw, c.token)
		var payload struct {
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &payload)
		retryAfter := strings.TrimSpace(res.Header.Get("Retry-After"))
		return &APIError{
			Status:             res.StatusCode,
			StatusCode:         res.StatusCode,
			Code:               strings.TrimSpace(payload.Code),
			Message:            strings.TrimSpace(payload.Error),
			Body:               body,
			RetryAfter:         retryAfter,
			RetryAfterDuration: parseRetryAfter(retryAfter, time.Now()),
		}
	}
	if target != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, target); err != nil {
			return &responseFormatError{Status: res.StatusCode, err: err}
		}
	}
	return nil
}

func safeAPIErrorBody(raw []byte, token string) string {
	body := strings.TrimSpace(string(raw))
	if token != "" {
		body = strings.ReplaceAll(body, token, "[REDACTED]")
	}
	const maxErrorBodyBytes = 4096
	if len(body) > maxErrorBodyBytes {
		body = body[:maxErrorBodyBytes] + "..."
	}
	return body
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil && retryAt.After(now) {
		return retryAt.Sub(now)
	}
	return 0
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

func wantsHelp(args []string) bool {
	return len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help"))
}

func singleID(usage string, args []string) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return "", errors.New("usage: " + usage)
	}
	return args[0], nil
}

func visitedValues(fs *flag.FlagSet, values map[string]any, names map[string]string) map[string]any {
	body := map[string]any{}
	fs.Visit(func(item *flag.Flag) {
		name := item.Name
		if renamed := names[name]; renamed != "" {
			name = renamed
		}
		body[name] = values[item.Name]
	})
	return body
}

func setQuery(q url.Values, key string, value string) {
	if strings.TrimSpace(value) != "" {
		q.Set(key, value)
	}
}

// requestBuildError marks a request that could not be formed at all, which in
// practice means WRKFLW_BASE_URL is not a usable address. It is permanent, so a
// caller that retries transient failures must not retry this.
type requestBuildError struct{ err error }

func (e *requestBuildError) Error() string {
	return fmt.Sprintf("WRKFLW_BASE_URL is not a usable address: %v", e.err)
}

func (e *requestBuildError) Unwrap() error { return e.err }

// responseFormatError marks a reply that was not the API's, such as an error
// page from a proxy served with a success status. Repeating the request will
// keep producing it, so callers treat it as final rather than transient.
type responseFormatError struct {
	Status int
	err    error
}

func (e *responseFormatError) Error() string {
	return fmt.Sprintf("wrkflw API %d returned a reply that is not JSON: %v", e.Status, e.err)
}

func (e *responseFormatError) Unwrap() error { return e.err }

func validStatus(status string) bool {
	switch status {
	case "new", "queued", "working", "needs_review", "done":
		return true
	default:
		return false
	}
}

func validPriority(priority string) bool {
	switch priority {
	case "", "p0", "p1", "p2", "p3":
		return true
	default:
		return false
	}
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, char := range value {
		switch index {
		case 8, 13, 18, 23:
			continue
		}
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') && !(char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}
