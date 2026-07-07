package ast

import (
	"encoding/json"
	"reflect"
	"testing"
)

func normalizeJSONVal(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		if len(val) == 0 {
			return nil
		}
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = normalizeJSONVal(item)
		}
		return out
	case map[string]interface{}:
		if len(val) == 0 {
			return nil
		}
		out := make(map[string]interface{})
		for k, item := range val {
			out[k] = normalizeJSONVal(item)
		}
		return out
	default:
		return val
	}
}

func assertJSONEquivalent(t *testing.T, origBytes, decBytes []byte, msg string) {
	t.Helper()
	var origVal, decVal interface{}
	if err := json.Unmarshal(origBytes, &origVal); err != nil {
		t.Fatalf("Failed to unmarshal original: %v", err)
	}
	if err := json.Unmarshal(decBytes, &decVal); err != nil {
		t.Fatalf("Failed to unmarshal decoded: %v", err)
	}

	normOrig := normalizeJSONVal(origVal)
	normDec := normalizeJSONVal(decVal)

	normOrigBytes, _ := json.Marshal(normOrig)
	normDecBytes, _ := json.Marshal(normDec)

	if !reflect.DeepEqual(normOrig, normDec) {
		t.Errorf("%s\nOriginal normalized: %s\nDecoded normalized:  %s", msg, string(normOrigBytes), string(normDecBytes))
	}
}

func TestCodecRoundtrip(t *testing.T) {
	// A complex Program containing almost all AST nodes
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "StructDecl",
				"name": "Point",
				"fields": [
					{"name": "x", "type": {"kind": "Int"}},
					{"name": "y", "type": {"kind": "Int"}}
				]
			},
			{
				"kind": "EnumDecl",
				"name": "Color",
				"variants": ["Red", "Green", "Blue"]
			},
			{
				"kind": "FunctionDecl",
				"name": "calculate",
				"params": [
					{"name": "p", "type": {"kind": "Point"}},
					{"name": "c", "type": {"kind": "Color"}},
					{"name": "arr", "type": {"kind": "Array", "elem": {"kind": "String"}}}
				],
				"returnType": {"kind": "Result", "okType": {"kind": "Int"}, "errType": {"kind": "String"}},
				"effects": ["pure"],
				"grant": ["io"],
				"body": [
					{
						"kind": "VarDecl",
						"name": "x",
						"type": {"kind": "Int"},
						"value": {"kind": "Literal", "valueType": "Int", "value": 42}
					},
					{
						"kind": "AssignStmt",
						"target": {"kind": "Ident", "name": "x"},
						"value": {"kind": "BinaryExpr", "op": "+", "left": {"kind": "Ident", "name": "x"}, "right": {"kind": "Literal", "valueType": "Int", "value": 1}}
					},
					{
						"kind": "IfStmt",
						"cond": {"kind": "Literal", "valueType": "Bool", "value": true},
						"then": [
							{"kind": "ExprStmt", "expr": {"kind": "CallExpr", "callee": "println", "args": [{"kind": "Ident", "name": "x"}]}}
						],
						"else": [
							{"kind": "BreakStmt"}
						]
					},
					{
						"kind": "WhileStmt",
						"cond": {"kind": "BinaryExpr", "op": "<", "left": {"kind": "Ident", "name": "x"}, "right": {"kind": "Literal", "valueType": "Int", "value": 100}},
						"body": [
							{"kind": "ContinueStmt"}
						]
					},
					{
						"kind": "ForStmt",
						"form": "range",
						"var": "i",
						"start": {"kind": "Literal", "valueType": "Int", "value": 0},
						"end": {"kind": "Literal", "valueType": "Int", "value": 10},
						"body": []
					},
					{
						"kind": "MatchExpr",
						"value": {"kind": "Ident", "name": "c"},
						"arms": [
							{
								"pattern": {"kind": "Literal", "valueType": "String", "value": "Red"},
								"body": [
									{"kind": "ReturnStmt", "value": {"kind": "Literal", "valueType": "Int", "value": 1}}
								]
							},
							{
								"pattern": {"kind": "Ident", "name": "_"},
								"body": [
									{"kind": "ReturnStmt", "value": {"kind": "Literal", "valueType": "Int", "value": 0}}
								]
							}
						]
					}
				]
			}
		]
	}`

	original, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Encode to binary
	binData, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode from binary
	decoded, err := Decode(binData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Check JSON representation equivalence
	origJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal(original) failed: %v", err)
	}
	decJSON, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal(decoded) failed: %v", err)
	}

	assertJSONEquivalent(t, origJSON, decJSON, "Decoded JSON does not match original JSON.")
}

func TestStableHashDifferentOrder(t *testing.T) {
	// Two JSON strings representing the same AST but with different key/field order
	src1 := `{
		"kind": "Program",
		"declarations": [{
			"kind": "FunctionDecl",
			"name": "f",
			"params": [],
			"returnType": {"kind": "Int"},
			"effects": [],
			"grant": [],
			"body": []
		}]
	}`

	src2 := `{
		"declarations": [{
			"body": [],
			"grant": [],
			"effects": [],
			"returnType": {"kind": "Int"},
			"params": [],
			"name": "f",
			"kind": "FunctionDecl"
		}],
		"kind": "Program"
	}`

	node1, err := Parse([]byte(src1))
	if err != nil {
		t.Fatalf("Parse src1 failed: %v", err)
	}

	node2, err := Parse([]byte(src2))
	if err != nil {
		t.Fatalf("Parse src2 failed: %v", err)
	}

	hash1, err := HashNode(node1)
	if err != nil {
		t.Fatalf("HashNode node1 failed: %v", err)
	}

	hash2, err := HashNode(node2)
	if err != nil {
		t.Fatalf("HashNode node2 failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Expected identical hash for identical AST with different construction JSON order, got %s vs %s", hash1, hash2)
	}
}

func TestCodecNewNodesRoundtrip(t *testing.T) {
	src := `{
		"kind": "Program",
		"declarations": [
			{
				"kind": "ImportDecl",
				"path": "./utils.xql.json",
				"as": "utils"
			},
			{
				"kind": "ClassDecl",
				"name": "User",
				"fields": [
					{"name": "id", "type": {"kind": "Int"}, "visibility": "private"},
					{"name": "name", "type": {"kind": "String"}, "visibility": "public"}
				]
			},
			{
				"kind": "FunctionDecl",
				"name": "testNewNodes",
				"params": [],
				"returnType": {"kind": "Void"},
				"effects": [],
				"grant": [],
				"body": [
					{
						"kind": "SwitchStmt",
						"value": {"kind": "Literal", "valueType": "Int", "value": 1},
						"cases": [
							{
								"value": {"kind": "Literal", "valueType": "Int", "value": 1},
								"body": []
							},
							{
								"value": null,
								"body": []
							}
						]
					},
					{
						"kind": "VarDecl",
						"name": "m",
						"type": {"kind": "Map", "keyType": {"kind": "String"}, "valueType": {"kind": "Int"}},
						"value": {
							"kind": "MapLiteral",
							"keyType": {"kind": "String"},
							"valueType": {"kind": "Int"},
							"entries": [
								{
									"key": {"kind": "Literal", "valueType": "String", "value": "a"},
									"value": {"kind": "Literal", "valueType": "Int", "value": 1}
								}
							]
						}
					},
					{
						"kind": "VarDecl",
						"name": "arr",
						"type": {"kind": "Array", "elem": {"kind": "Int"}},
						"value": {
							"kind": "ArrayLiteral",
							"elemType": {"kind": "Int"},
							"elements": [
								{"kind": "Literal", "valueType": "Int", "value": 1},
								{"kind": "Literal", "valueType": "Int", "value": 2}
							]
						}
					}
				]
			}
		]
	}`

	original, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse new nodes failed: %v", err)
	}

	binData, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode new nodes failed: %v", err)
	}

	decoded, err := Decode(binData)
	if err != nil {
		t.Fatalf("Decode new nodes failed: %v", err)
	}

	origJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal(original) failed: %v", err)
	}
	decJSON, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal(decoded) failed: %v", err)
	}

	assertJSONEquivalent(t, origJSON, decJSON, "Decoded JSON does not match original JSON for new nodes.")
}
