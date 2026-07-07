package check

import (
	"encoding/json"
	"errors"

	"xiaoqinli/ast"
	"xiaoqinli/vfs"
)

// RunAll performs all static checks on the typed AST:
// 1. Type checking (types + return values + operator compatibility)
// 2. Effect inference and verification (@effects purity)
// 3. Capability verification (@grant inheritance)
//
// Any check failure returns an error; all checks must pass before codegen.
type Diagnostic struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Location     string `json:"location,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

type WorkspaceError struct {
	Diagnostics []Diagnostic
}

func (we WorkspaceError) Error() string {
	data, _ := json.MarshalIndent(we.Diagnostics, "", "  ")
	return string(data)
}

func RunAll(root ast.Node) error {
	return RunAllInWorkspace(root, "", nil)
}

func RunAllInWorkspace(root ast.Node, currentFile string, ws *vfs.Workspace) error {
	if root == nil {
		return errors.New("root node is nil")
	}

	tc := NewTypeChecker()
	tc.currentFile = currentFile
	tc.workspace = ws

	_ = tc.Check(root)
	_ = CheckEffectsWithTC(root, tc)
	_ = CheckCapabilitiesWithTC(root, tc)

	if len(tc.errors) > 0 {
		return WorkspaceError{Diagnostics: tc.Diagnostics}
	}

	return nil
}
