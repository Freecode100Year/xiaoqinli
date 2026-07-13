package ast

import (
	"encoding/json"
	"fmt"
)

// Node is the interface all AST nodes implement.
type Node interface {
	Kind() string
}

// LocationInfo provides structured position data for diagnostics.
type LocationInfo struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line"`   // 1-indexed
	Column int    `json:"column"` // 1-indexed
	Length int    `json:"length"` // range length
}

// TypeExpr represents a type reference in the AST.
type TypeExpr struct {
	KindName string    `json:"kind"`
	Elem     *TypeExpr `json:"elem,omitempty"`     // Array<T>, Option<T>
	KeyType  *TypeExpr `json:"keyType,omitempty"`   // Map<K,V> key type
	OkType   *TypeExpr `json:"okType,omitempty"`
	ErrType  *TypeExpr `json:"errType,omitempty"`
}

// Param represents a function parameter.
type Param struct {
	Name string   `json:"name"`
	Type TypeExpr `json:"type"`
}

// --- Top-level ---

// Program is the top-level node containing multiple declarations.
type Program struct {
	Decls []Node
}

func (*Program) Kind() string { return "Program" }

// ImportDecl represents an import declaration.
type ImportDecl struct {
	Path string
	As   string
}

func (*ImportDecl) Kind() string { return "ImportDecl" }

// --- Declarations & Statements ---

// FunctionDecl represents a function declaration.
type FunctionDecl struct {
	Name       string
	Params     []Param
	ReturnType TypeExpr
	Effects    []string
	Grant      []string
	Body       []Node
}

func (*FunctionDecl) Kind() string { return "FunctionDecl" }

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value Node
}

func (*ReturnStmt) Kind() string { return "ReturnStmt" }

// VarDecl represents a variable declaration.
type VarDecl struct {
	Name  string
	Type  TypeExpr
	Value Node
}

func (*VarDecl) Kind() string { return "VarDecl" }

// AssignStmt represents an assignment statement.
// Target can be Ident, IndexExpr, or MemberExpr.
type AssignStmt struct {
	Target Node
	Value  Node
}

func (*AssignStmt) Kind() string { return "AssignStmt" }

// IfStmt represents an if/else statement.
type IfStmt struct {
	Cond Node
	Then []Node
	Else []Node
}

func (*IfStmt) Kind() string { return "IfStmt" }

// WhileStmt represents a while loop.
type WhileStmt struct {
	Cond Node
	Body []Node
}

func (*WhileStmt) Kind() string { return "WhileStmt" }

// ForStmt represents a for loop with two forms:
//   - "range": iterates var from start to end (exclusive)
//   - "each":  iterates var over each element of iterable
type ForStmt struct {
	Form     string // "range" or "each"
	Var      string // loop variable name
	Start    Node   // range form only
	End      Node   // range form only
	Iterable Node   // each form only
	Body     []Node
}

func (*ForStmt) Kind() string { return "ForStmt" }

// BreakStmt represents a break statement inside a loop.
type BreakStmt struct{}

func (*BreakStmt) Kind() string { return "BreakStmt" }

// ContinueStmt represents a continue statement inside a loop.
type ContinueStmt struct{}

func (*ContinueStmt) Kind() string { return "ContinueStmt" }

// ExprStmt wraps an expression used as a statement.
type ExprStmt struct {
	Expr Node
}

func (*ExprStmt) Kind() string { return "ExprStmt" }

// StructField represents a field in a struct declaration.
type StructField struct {
	Name       string   `json:"name"`
	Type       TypeExpr `json:"type"`
	Visibility string   `json:"visibility,omitempty"` // "public" or "private"
}

// StructDecl represents a struct type declaration.
type StructDecl struct {
	Name   string
	Fields []StructField
}

func (*StructDecl) Kind() string { return "StructDecl" }

// EnumDecl represents an enum type declaration with simple string variants.
type EnumDecl struct {
	Name     string
	Variants []string
}

func (*EnumDecl) Kind() string { return "EnumDecl" }

// MatchArm represents one arm of a match expression.
type MatchArm struct {
	Pattern Node   // Literal or Ident (name "_" = wildcard)
	Body    []Node
}

// MatchExpr represents a match/switch expression.
type MatchExpr struct {
	Value Node
	Arms  []MatchArm
}

func (*MatchExpr) Kind() string { return "MatchExpr" }

// --- Expressions ---

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	Op    string
	Left  Node
	Right Node
}

func (*BinaryExpr) Kind() string { return "BinaryExpr" }

// UnaryExpr represents a unary operation.
type UnaryExpr struct {
	Op      string
	Operand Node
}

func (*UnaryExpr) Kind() string { return "UnaryExpr" }

// CallExpr represents a function call.
type CallExpr struct {
	Callee string
	Args   []Node
}

func (*CallExpr) Kind() string { return "CallExpr" }

// Literal represents a literal value.
type Literal struct {
	ValueType string      // "String", "Int", "Float", "Bool"
	Value     interface{} // actual value
}

func (*Literal) Kind() string { return "Literal" }

// Ident represents an identifier reference.
type Ident struct {
	Name string
}

func (*Ident) Kind() string { return "Ident" }

// MemberExpr represents member access (obj.field).
type MemberExpr struct {
	Object Node
	Field  string
}

func (*MemberExpr) Kind() string { return "MemberExpr" }

// StructFieldInit represents a field initialization in a struct literal.
type StructFieldInit struct {
	Name  string
	Value Node
}

// StructLit represents a struct literal expression.
type StructLit struct {
	TypeName string
	Fields   []StructFieldInit
}

func (*StructLit) Kind() string { return "StructLit" }

// ArrayLit represents an array/list literal expression.
type ArrayLit struct {
	ElemType TypeExpr
	Elements []Node
}

func (*ArrayLit) Kind() string { return "ArrayLit" }

// IndexExpr represents an index/subscript access (target[index]).
type IndexExpr struct {
	Target Node
	Index  Node
}

func (*IndexExpr) Kind() string { return "IndexExpr" }

// IfExpr represents a ternary/conditional expression (value, not statement).
type IfExpr struct {
	Cond Node
	Then Node
	Else Node
}

func (*IfExpr) Kind() string { return "IfExpr" }

// NewExpr represents a constructor call (new Callee(Args...))
type NewExpr struct {
	Callee string
	Args   []Node
}

func (*NewExpr) Kind() string { return "NewExpr" }

// AwaitExpr represents an await operation (await Expr)
type AwaitExpr struct {
	Expr Node
}

func (*AwaitExpr) Kind() string { return "AwaitExpr" }

// ClassField represents a field in a class declaration.
type ClassField struct {
	Name       string   `json:"name"`
	Type       TypeExpr `json:"type"`
	Visibility string   `json:"visibility,omitempty"` // "public" or "private"
}

// ClassDecl represents a class type declaration.
type ClassDecl struct {
	Name   string
	Fields []ClassField
}

func (*ClassDecl) Kind() string { return "ClassDecl" }

// SwitchCase represents a case in a switch statement.
type SwitchCase struct {
	Value Node   // nil for default case
	Body  []Node
}

// SwitchStmt represents a switch statement.
type SwitchStmt struct {
	Value Node
	Cases []SwitchCase
}

func (*SwitchStmt) Kind() string { return "SwitchStmt" }

// MapEntry represents a key-value entry in a MapLiteral.
type MapEntry struct {
	Key   Node
	Value Node
}

// MapLiteral represents a map literal.
type MapLiteral struct {
	KeyType   TypeExpr
	ValueType TypeExpr
	Entries   []MapEntry
}

func (*MapLiteral) Kind() string { return "MapLiteral" }

// ArrayLiteral represents an array literal.
type ArrayLiteral struct {
	ElemType TypeExpr
	Elements []Node
}

func (*ArrayLiteral) Kind() string { return "ArrayLiteral" }

// Lambda represents an anonymous function / closure expression.
type Lambda struct {
	Params     []Param
	ReturnType TypeExpr
	Body       []Node
}

func (*Lambda) Kind() string { return "Lambda" }

// ===================== JSON Parsing ================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================================= laze(" laze(\" laze(\" l")
}

func parseNode(raw map[string]interface{}) (Node, error) {
	kind, ok := raw["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("XQL_E101: node missing 'kind' field으로 l")
	}
	switch kind {
	case "Program":
		return parseProgram(raw)
	case "ImportDecl":
		return parseImportDecl(raw)
	case "FunctionDecl":
		return parseFunctionDecl(raw)
	case "ReturnStmt":
		return parseReturnStmt(raw)
	case "VarDecl":
		return parseVarDecl(raw)
	case "AssignStmt":
		return parseAssignStmt(raw)
	case "IfStmt":
		return parseIfStmt(raw)
	case "WhileStmt":
		return parseWhileStmt(raw)
	case "ForStmt":
		return parseForStmt(raw)
	case "BreakStmt":
		return &BreakStmt{}, nil
	case "ContinueStmt":
		return &ContinueStmt{}, nil
	case "ExprStmt":
		return parseExprStmt(raw)
	case "StructDecl":
		return parseStructDecl(raw)
	case "ClassDecl":
		return parseClassDecl(raw)
	case "EnumDecl":
		return parseEnumDecl(raw)
	case "MatchExpr":
		return parseMatchExpr(raw)
	case "SwitchStmt":
		return parseSwitchStmt(raw)
	case "StructLit":
		return parseStructLit(raw)
	case "ArrayLit":
		return parseArrayLit(raw)
	case "ArrayLiteral":
		return parseArrayLiteral(raw)
	case "MapLiteral":
		return parseMapLiteral(raw)
	case "IndexExpr":
		return parseIndexExpr(raw)
	case "IfExpr":
		return parseIfExpr(raw)
	case "Lambda":
		return parseLambda(raw)
	case "BinaryExpr":
		return parseBinaryExpr(raw)
	case "UnaryExpr":
		return parseUnaryExpr(raw)
	case "CallExpr":
		return parseCallExpr(raw)
	case "Literal":
		return parseLiteral(raw)
	case "Ident":
		return parseIdent(raw)
	case "MemberExpr":
		return parseMemberExpr(raw)
	case "NewExpr":
		return parseNewExpr(raw)
	case "AwaitExpr":
		return parseAwaitExpr(raw)
	default:
		return nil, fmt.Errorf("XQL_E101: unknown node kind: %s", kind)
	}
}

func parseNodeList(raw []interface{}) ([]Node, error) {
	nodes := make([]Node, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("XQL_E101: element %d is not an object", i)
		}
		n, err := parseNode(m)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func parseChildNode(raw map[string]interface{}, field string) (Node, error) {
	v, ok := raw[field]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("XQL_E101: '%s' is not an object", field)
	}
	return parseNode(m)
}

func parseTypeExpr(raw interface{}) (TypeExpr, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return TypeExpr{}, fmt.Errorf("XQL_E101: type is not an object")
	}
	te := TypeExpr{}
	te.KindName, _ = m["kind"].(string)
	if te.KindName == "" {
		return TypeExpr{}, fmt.Errorf("XQL_E101: type missing 'kind'")
	}
	if elem, ok := m["elem"]; ok {
		e, err := parseTypeExpr(elem)
		if err != nil {
			return TypeExpr{}, err
		}
		te.Elem = &e
	}
	if keyT, ok := m["keyType"]; ok {
		k, err := parseTypeExpr(keyT)
		if err != nil {
			return TypeExpr{}, err
		}
		te.KeyType = &k
	}
	if okT, ok := m["okType"]; ok {
		o, err := parseTypeExpr(okT)
		if err != nil {
			return TypeExpr{}, err
		}
		te.OkType = &o
	}
	if errT, ok := m["errType"]; ok {
		e, err := parseTypeExpr(errT)
		if err != nil {
			return TypeExpr{}, err
		}
		te.ErrType = &e
	}
	return te, nil
}

func parseStringList(raw interface{}) []string {
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
