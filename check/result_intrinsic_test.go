package check

import (
	"strings"
	"testing"
)

// resultIntrinsicProgram builds a program whose main calls method on a
// Result-typed local produced by makeResult.
func resultIntrinsicProgram(method string) string {
	return `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "FunctionDecl",
				"name": "makeResult",
				"params": [],
				"returnType": {"kind": "Result", "okType": {"kind": "Int"}, "errType": {"kind": "String"}},
				"effects": [],
				"grant": [],
				"body": [{
					"kind": "ReturnStmt",
					"value": {
						"kind": "CallExpr",
						"callee": "Result.ok",
						"args": [{"kind": "Literal", "valueType": "Int", "value": 1}]
					}
				}]
			},
			{
				"kind": "FunctionDecl",
				"name": "main",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "VarDecl",
						"name": "res",
						"type": {"kind": "Result", "okType": {"kind": "Int"}, "errType": {"kind": "String"}},
						"value": {"kind": "CallExpr", "callee": "makeResult", "args": []}
					},
					{
						"kind": "ExprStmt",
						"expr": {"kind": "CallExpr", "callee": "res.` + method + `", "args": []}
					}
				]
			}
		]
	}`
}

// TestStrictCapabilityAllowsResultIntrinsics ensures unwrap/unwrapErr, which
// the type checker already accepts on Result receivers, do not trip the
// strict capability check. They are pure and require no grant.
func TestStrictCapabilityAllowsResultIntrinsics(t *testing.T) {
	for _, method := range []string{"unwrap", "unwrapErr"} {
		t.Run(method, func(t *testing.T) {
			root := mustParse(t, resultIntrinsicProgram(method))
			if err := CheckCapabilitiesStrict(root); err != nil {
				t.Errorf("expected strict check to accept %q, got: %v", method, err)
			}
		})
	}
}

// TestStrictCapabilityAllowsResultConstructors covers Result.ok / Result.err,
// which TypeChecker.inferType also treats as built-ins.
func TestStrictCapabilityAllowsResultConstructors(t *testing.T) {
	for _, ctor := range []string{"Result.ok", "Result.err"} {
		t.Run(ctor, func(t *testing.T) {
			src := `{
				"kind": "Program",
				"declarations": [{
					"kind": "FunctionDecl",
					"name": "main",
					"params": [],
					"returnType": {"kind": "Void"},
					"effects": [],
					"grant": [],
					"body": [{
						"kind": "ExprStmt",
						"expr": {
							"kind": "CallExpr",
							"callee": "` + ctor + `",
							"args": [{"kind": "Literal", "valueType": "Int", "value": 1}]
						}
					}]
				}]
			}`
			root := mustParse(t, src)
			if err := CheckCapabilitiesStrict(root); err != nil {
				t.Errorf("expected strict check to accept %q, got: %v", ctor, err)
			}
		})
	}
}

// TestStrictCapabilityStillRejectsUnknownMethod guards against the intrinsic
// allowance widening into a blanket exemption for dotted calls.
func TestStrictCapabilityStillRejectsUnknownMethod(t *testing.T) {
	root := mustParse(t, resultIntrinsicProgram("readSecretFile"))
	err := CheckCapabilitiesStrict(root)
	if err == nil {
		t.Fatal("expected strict check to reject an unknown dotted call")
	}
	if !strings.Contains(err.Error(), "XQL_E303") {
		t.Errorf("expected XQL_E303, got: %v", err)
	}
}
