package compiler

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"xiaoqinli/internal/e2e"
)

// Two backends emit a bundle rather than a program: chrome writes a Chrome
// extension and shortcut writes an Apple Shortcuts workflow. Neither can be run
// against an expected stdout, so the conformance corpus cannot reach them, and
// both sat at the smoke tier — "codegen returned bytes, and nothing has looked
// at them".
//
// That is true of the *program*, and it was never true of the parts. A Chrome
// extension is a manifest and some JavaScript: JSON has a parser and JavaScript
// has node --check. Being unable to load the extension into a browser is not a
// reason to leave the JavaScript unread.
//
// What this does not claim: that Chrome accepts the extension, or that the
// Shortcuts app accepts the workflow. Only their consumers can say that, and
// neither runs in CI.

// TestChromeBundle reads the extension the way its own tooling would: the
// manifest as JSON, the scripts through a JavaScript parser.
func TestChromeBundle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain-driven checks in -short mode")
	}

	corpus := exampleCorpus(t)
	// chrome returns its bundle as one JSON object keyed by filename, and the
	// compiler unpacks that to disk. Reading the directory rather than the
	// return value checks the unpacking too, which is the step that has to get
	// the path traversal guard right.
	dir := t.TempDir()
	res := CompileFromFile(corpus["hello.xql.json"], "chrome", dir)
	if !res.Success {
		t.Fatalf("chrome declined hello.xql.json: %s", res.Error)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("the bundle has no manifest.json, which is the one file Chrome requires: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifest.json is not valid JSON: %v\n%s", err, raw)
	}

	// Manifest V3 refuses to load without these three, whatever else is present.
	for _, key := range []string{"manifest_version", "name", "version"} {
		if _, ok := manifest[key]; !ok {
			t.Errorf("manifest.json has no %q; Chrome will not load the extension", key)
		}
	}
	if v, _ := manifest["manifest_version"].(float64); v != 3 {
		t.Errorf("manifest_version is %v, and this backend advertises MV3", manifest["manifest_version"])
	}

	// A popup naming a file the bundle does not contain is a broken extension
	// that reading the manifest alone cannot show.
	if action, ok := manifest["action"].(map[string]any); ok {
		if popup, ok := action["default_popup"].(string); ok {
			if _, err := os.Stat(filepath.Join(dir, popup)); err != nil {
				t.Errorf("the manifest points default_popup at %q, which the bundle does not contain", popup)
			}
		}
	}

	node := e2e.FirstWorking("node")
	if node == "" {
		e2e.Missing(t, "node is not on PATH, so the extension's JavaScript was generated but never parsed")
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read the bundle: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	checked := 0
	for _, name := range names {
		if !strings.HasSuffix(name, ".js") {
			continue
		}
		// --check parses and reports syntax errors without running a line, the
		// same bargain the compiled tier makes everywhere else.
		out, err := exec.Command(node, "--check", filepath.Join(dir, name)).CombinedOutput()
		if err != nil {
			t.Errorf("node rejects %s: %v\n%s", name, err, out)
			continue
		}
		checked++
	}
	if checked == 0 {
		t.Error("the bundle contains no JavaScript at all, so nothing here was parsed")
	}
}

// TestShortcutBundle is what can honestly be said about the shortcut backend.
//
// The registry called its output a plist for a long time. It is JSON, and the
// note was never checked against the bytes — which is the same species of
// mistake the verification tiers exist to prevent, one level up.
//
// Nothing here establishes that the Shortcuts app would accept the workflow.
// That app runs on Apple platforms only, has no command-line validator, and so
// this backend stays at the smoke tier: the structure is checked, the meaning
// is not.
func TestShortcutBundle(t *testing.T) {
	corpus := exampleCorpus(t)
	res := CompileFromFile(corpus["hello.xql.json"], "shortcut", "")
	if !res.Success {
		t.Fatalf("shortcut declined hello.xql.json: %s", res.Error)
	}

	var workflow struct {
		Actions []struct {
			Identifier string `json:"WFWorkflowActionIdentifier"`
		} `json:"WFWorkflowActions"`
	}
	if err := json.Unmarshal(res.Code, &workflow); err != nil {
		t.Fatalf("the shortcut is not valid JSON: %v\n%s", err, res.Code)
	}
	if len(workflow.Actions) == 0 {
		t.Fatal("the workflow has no actions, so the shortcut does nothing")
	}
	for i, a := range workflow.Actions {
		if !strings.HasPrefix(a.Identifier, "is.workflow.actions.") {
			t.Errorf("action %d has identifier %q; Shortcuts actions are namespaced is.workflow.actions.*",
				i, a.Identifier)
		}
	}
}

// TestShortcutLoopVariableIsBound is the one thing about a generated Repeat
// that can be checked without the app: that the loop variable exists.
//
// It did not. A Repeat publishes Repeat Index and Repeat Item and this backend
// read neither, so the body of every loop it ever emitted referred to a
// variable no action set — loop.xql.json's `i`, nested_loop's `i` and `j`. The
// smoke tier is exactly the tier that cannot notice: the JSON parsed, every
// action was namespaced, and the workflow computed with an empty variable.
func TestShortcutLoopVariableIsBound(t *testing.T) {
	corpus := exampleCorpus(t)

	cases := []struct {
		example string
		open    string // the action that opens the loop
		source  string // the variable Shortcuts publishes to its body
		bound   string // the variable the XQL program reads
	}{
		{"loop.xql.json", "is.workflow.actions.repeat.count", "Repeat Index", "i"},
		{"for_each.xql.json", "is.workflow.actions.repeat.each", "Repeat Item", "n"},
	}

	for _, tc := range cases {
		t.Run(tc.example, func(t *testing.T) {
			res := CompileFromFile(corpus[tc.example], "shortcut", "")
			if !res.Success {
				t.Fatalf("shortcut declined %s: %s", tc.example, res.Error)
			}

			var workflow struct {
				Actions []struct {
					Identifier string                 `json:"WFWorkflowActionIdentifier"`
					Params     map[string]interface{} `json:"WFWorkflowActionParameters"`
				} `json:"WFWorkflowActions"`
			}
			if err := json.Unmarshal(res.Code, &workflow); err != nil {
				t.Fatalf("the shortcut is not valid JSON: %v", err)
			}

			open := -1
			for i, a := range workflow.Actions {
				if a.Identifier == tc.open && a.Params["WFControlFlowMode"] == float64(0) {
					open = i
					break
				}
			}
			if open < 0 {
				t.Fatalf("no %s action opens a loop", tc.open)
			}

			// The binding is the first thing in the body: read the published
			// variable, adjust it if the range does not start at 1, store it
			// under the name the program uses.
			body := workflow.Actions[open+1:]
			if len(body) == 0 || body[0].Identifier != "is.workflow.actions.getvariable" {
				t.Fatalf("the loop body does not start by reading a variable, so %q is never bound", tc.bound)
			}
			if got := shortcutVarName(body[0].Params["WFVariable"]); got != tc.source {
				t.Errorf("loop body reads %q, want the published %q", got, tc.source)
			}

			set := false
			for _, a := range body[:min(4, len(body))] {
				if a.Identifier == "is.workflow.actions.setvariable" && a.Params["WFVariableName"] == tc.bound {
					set = true
					break
				}
			}
			if !set {
				t.Errorf("no action near the top of the loop sets %q, so the body reads an empty variable", tc.bound)
			}
		})
	}
}

// shortcutVarName digs the name out of a WFTextTokenAttachment, which is how
// every variable reference in a workflow is serialized.
func shortcutVarName(ref interface{}) string {
	m, ok := ref.(map[string]interface{})
	if !ok {
		return ""
	}
	value, ok := m["Value"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := value["VariableName"].(string)
	return name
}
