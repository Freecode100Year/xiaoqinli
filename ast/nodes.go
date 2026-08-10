package ast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
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
	Elem     *TypeExpr `json:"elem,omitempty"`    // Array<T>, Option<T>
	KeyType  *TypeExpr `json:"keyType,omitempty"` // Map<K,V> key type
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

// ExternDecl declares a function the host platform provides and the compiler
// therefore never emits: `fetch`, `time.Sleep`, `document.createElement`.
//
// Without it every call to a platform API is an unresolved symbol, so the type
// checker rejects the program and the capability checker cannot say anything
// about the one place that actually reaches the outside world. Declaring the
// signature turns a host call into a checked, capability-carrying edge.
//
// Name is matched verbatim against CallExpr.Callee, so dotted and chained
// names are declared exactly as they are called.
type ExternDecl struct {
	Name       string
	Params     []Param
	ReturnType TypeExpr
	Effects    []string
	// Grant lists the capabilities a caller must hold to call this extern.
	Grant []string
	// Targets restricts the extern to the backends whose host actually
	// provides it. Empty means every target.
	Targets []string
	// Method declares a method on a host object rather than a global: it
	// matches any call whose final dotted segment is Name, such as
	// `res.json()` or `hud.classList.add()`. The receiver is a runtime value
	// the compiler cannot type, so only the method name is matched — the
	// declared grant is still enforced at every call site.
	Method bool
	// HasParams distinguishes `"params": []` (a checked zero-arity signature)
	// from an omitted params field (arity and argument types unchecked).
	HasParams bool
}

func (*ExternDecl) Kind() string { return "ExternDecl" }

// SignatureEquals reports whether two extern declarations describe the same
// host function. Declaring the same extern in several modules is normal — the
// host provides one `fetch` no matter how many files mention it — so merging
// only has to reject declarations that disagree.
func (e *ExternDecl) SignatureEquals(o *ExternDecl) bool {
	if e == nil || o == nil {
		return e == o
	}
	if e.Name != o.Name || e.HasParams != o.HasParams || e.Method != o.Method {
		return false
	}
	if !sameTypeExpr(e.ReturnType, o.ReturnType) {
		return false
	}
	if len(e.Params) != len(o.Params) {
		return false
	}
	for i := range e.Params {
		if !sameTypeExpr(e.Params[i].Type, o.Params[i].Type) {
			return false
		}
	}
	return sameStringSet(e.Effects, o.Effects) &&
		sameStringSet(e.Grant, o.Grant) &&
		sameStringSet(e.Targets, o.Targets)
}

// sameTypeExpr compares two type expressions in full. Comparing only KindName
// would call Array<Int> and Array<String> the same type, letting two modules
// declare incompatible versions of one host function without complaint.
func sameTypeExpr(a, b TypeExpr) bool {
	if a.KindName != b.KindName {
		return false
	}
	return sameTypeExprPtr(a.Elem, b.Elem) &&
		sameTypeExprPtr(a.KeyType, b.KeyType) &&
		sameTypeExprPtr(a.OkType, b.OkType) &&
		sameTypeExprPtr(a.ErrType, b.ErrType)
}

func sameTypeExprPtr(a, b *TypeExpr) bool {
	if a == nil || b == nil {
		return a == b
	}
	return sameTypeExpr(*a, *b)
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

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
	Pattern Node // Literal or Ident (name "_" = wildcard)
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
//
// `/` between two Ints is integer division, truncating toward zero: 7 / 2 is 3.
// Saying so is not pedantry. Roughly a third of the target languages make `/`
// float division — Python, JavaScript, Perl, awk, Lua, PHP, Julia, Dart,
// Elixir, Groovy and PowerShell — and Haskell and Zig refuse `/` on integers
// altogether. A backend that passes the operator straight through inherits
// whichever meaning its language happens to have, and `7 / 2` printed both 3
// and 3.5 across the matrix until each of those emitted its language's integer
// division instead.
//
// Truncation is the majority rule (C, Go, Java, Rust, Swift, and the div/quot
// functions the rest reach for). Three backends floor instead, which differs
// only when exactly one operand is negative: py and lua use `//`, and ruby's
// `/` floors natively. That gap is not covered by the conformance corpus and
// should be treated as unspecified rather than as working.
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
	Value Node // nil for default case
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

// ===================== JSON Parsing =====================

// utf8BOM is the byte-order mark some editors and Windows tools (PowerShell's
// `-Encoding utf8`, Notepad's "UTF-8" save option) prepend to text files.
// encoding/json treats it as invalid input rather than stripping it.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Parse parses .xql.json bytes into a typed AST tree.
func Parse(data []byte) (Node, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("XQL_E101: invalid JSON: %w", err)
	}
	return parseNode(raw)
}

func parseNode(raw map[string]interface{}, depth ...int) (Node, error) {
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	if curDepth > MaxDecodeDepth {
		return nil, fmt.Errorf("XQL_E413: max decode depth exceeded %d > %d", curDepth, MaxDecodeDepth)
	}
	kind, ok := raw["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("XQL_E101: node missing 'kind' field")
	}
	switch kind {
	case "Program":
		return parseProgram(raw, curDepth)
	case "ImportDecl":
		return parseImportDecl(raw)
	case "ExternDecl":
		return parseExternDecl(raw, curDepth)
	case "FunctionDecl":
		return parseFunctionDecl(raw, curDepth)
	case "ReturnStmt":
		return parseReturnStmt(raw, curDepth)
	case "VarDecl":
		return parseVarDecl(raw, curDepth)
	case "AssignStmt":
		return parseAssignStmt(raw, curDepth)
	case "IfStmt":
		return parseIfStmt(raw, curDepth)
	case "WhileStmt":
		return parseWhileStmt(raw, curDepth)
	case "ForStmt":
		return parseForStmt(raw, curDepth)
	case "BreakStmt":
		return &BreakStmt{}, nil
	case "ContinueStmt":
		return &ContinueStmt{}, nil
	case "ExprStmt":
		return parseExprStmt(raw, curDepth)
	case "StructDecl":
		return parseStructDecl(raw)
	case "ClassDecl":
		return parseClassDecl(raw)
	case "EnumDecl":
		return parseEnumDecl(raw)
	case "MatchExpr":
		return parseMatchExpr(raw, curDepth)
	case "SwitchStmt":
		return parseSwitchStmt(raw, curDepth)
	case "StructLit":
		return parseStructLit(raw, curDepth)
	case "ArrayLit", "ArrayLiteral":
		// ArrayLit and ArrayLiteral are the same node (ElemType + Elements)
		// under two historical spellings. Normalizing both to *ArrayLit here
		// means every backend that implements one implements both, instead of
		// requiring each of the 45 codegen backends to carry two identical
		// emit functions to stay in sync.
		return parseArrayLit(raw, curDepth)
	case "MapLiteral":
		return parseMapLiteral(raw, curDepth)
	case "IndexExpr":
		return parseIndexExpr(raw, curDepth)
	case "IfExpr":
		return parseIfExpr(raw, curDepth)
	case "Lambda":
		return parseLambda(raw, curDepth)
	case "BinaryExpr":
		return parseBinaryExpr(raw, curDepth)
	case "UnaryExpr":
		return parseUnaryExpr(raw, curDepth)
	case "CallExpr":
		return parseCallExpr(raw, curDepth)
	case "Literal":
		return parseLiteral(raw)
	case "Ident":
		return parseIdent(raw)
	case "MemberExpr":
		return parseMemberExpr(raw, curDepth)
	case "NewExpr":
		return parseNewExpr(raw, curDepth)
	case "AwaitExpr":
		return parseAwaitExpr(raw, curDepth)
	default:
		return nil, fmt.Errorf("XQL_E101: unknown node kind: %s", kind)
	}
}

func parseNodeList(raw []interface{}, depth ...int) ([]Node, error) {
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	nodes := make([]Node, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("XQL_E101: element %d is not an object", i)
		}
		n, err := parseNode(m, curDepth+1)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func parseChildNode(raw map[string]interface{}, field string, depth ...int) (Node, error) {
	v, ok := raw[field]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("XQL_E101: '%s' is not an object", field)
	}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	return parseNode(m, curDepth+1)
}

func parseTypeExpr(raw interface{}, depth ...int) (TypeExpr, error) {
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	if curDepth > MaxTypeDepth {
		return TypeExpr{}, fmt.Errorf("XQL_E413: max type depth exceeded %d > %d", curDepth, MaxTypeDepth)
	}
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
		e, err := parseTypeExpr(elem, curDepth+1)
		if err != nil {
			return TypeExpr{}, err
		}
		te.Elem = &e
	}
	if keyT, ok := m["keyType"]; ok {
		k, err := parseTypeExpr(keyT, curDepth+1)
		if err != nil {
			return TypeExpr{}, err
		}
		te.KeyType = &k
	}
	if okT, ok := m["okType"]; ok {
		o, err := parseTypeExpr(okT, curDepth+1)
		if err != nil {
			return TypeExpr{}, err
		}
		te.OkType = &o
	}
	if errT, ok := m["errType"]; ok {
		e, err := parseTypeExpr(errT, curDepth+1)
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

// --- Individual node parsers ---

func parseProgram(raw map[string]interface{}, depth ...int) (*Program, error) {
	decls, ok := raw["declarations"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("XQL_E101: Program missing 'declarations'")
	}
	nodes, err := parseNodeList(decls, depth...)
	if err != nil {
		return nil, err
	}
	return &Program{Decls: nodes}, nil
}

func parseImportDecl(raw map[string]interface{}) (*ImportDecl, error) {
	id := &ImportDecl{}
	id.Path, _ = raw["path"].(string)
	id.As, _ = raw["as"].(string)
	return id, nil
}

// parseParams decodes a "params" array. The second result reports whether the
// field was present at all, which ExternDecl uses to tell a declared zero-arity
// signature from an unchecked one.
func parseParams(raw map[string]interface{}, depth ...int) ([]Param, bool, error) {
	params, ok := raw["params"].([]interface{})
	if !ok {
		return nil, false, nil
	}
	out := make([]Param, 0, len(params))
	for _, p := range params {
		pm, ok := p.(map[string]interface{})
		if !ok {
			return nil, true, fmt.Errorf("XQL_E101: param is not an object")
		}
		param := Param{}
		param.Name, _ = pm["name"].(string)
		if t, ok := pm["type"]; ok {
			te, err := parseTypeExpr(t, depth...)
			if err != nil {
				return nil, true, err
			}
			param.Type = te
		}
		out = append(out, param)
	}
	return out, true, nil
}

func parseExternDecl(raw map[string]interface{}, depth ...int) (*ExternDecl, error) {
	ed := &ExternDecl{}
	ed.Name, _ = raw["name"].(string)
	if ed.Name == "" {
		return nil, fmt.Errorf("XQL_E101: ExternDecl missing 'name'")
	}

	params, hasParams, err := parseParams(raw, depth...)
	if err != nil {
		return nil, err
	}
	ed.Params, ed.HasParams = params, hasParams

	if rt, ok := raw["returnType"]; ok {
		te, err := parseTypeExpr(rt, depth...)
		if err != nil {
			return nil, err
		}
		ed.ReturnType = te
	}

	ed.Effects = parseStringList(raw["effects"])
	ed.Grant = parseStringList(raw["grant"])
	ed.Targets = parseStringList(raw["targets"])
	ed.Method, _ = raw["method"].(bool)

	if ed.Method && strings.Contains(ed.Name, ".") {
		return nil, fmt.Errorf(
			"XQL_E101: extern method %q must be named by the method alone; the receiver is not part of the name", ed.Name)
	}
	if _, hasBody := raw["body"]; hasBody {
		return nil, fmt.Errorf("XQL_E101: ExternDecl %q must not have a body; the host provides the implementation", ed.Name)
	}
	return ed, nil
}

func parseFunctionDecl(raw map[string]interface{}, depth ...int) (*FunctionDecl, error) {
	fd := &FunctionDecl{}
	fd.Name, _ = raw["name"].(string)
	if fd.Name == "" {
		return nil, fmt.Errorf("XQL_E101: FunctionDecl missing 'name'")
	}

	params, _, err := parseParams(raw, depth...)
	if err != nil {
		return nil, err
	}
	fd.Params = params

	if rt, ok := raw["returnType"]; ok {
		te, err := parseTypeExpr(rt, depth...)
		if err != nil {
			return nil, err
		}
		fd.ReturnType = te
	}

	fd.Effects = parseStringList(raw["effects"])
	fd.Grant = parseStringList(raw["grant"])

	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body, depth...)
		if err != nil {
			return nil, err
		}
		fd.Body = nodes
	}
	return fd, nil
}

func parseReturnStmt(raw map[string]interface{}, depth ...int) (*ReturnStmt, error) {
	rs := &ReturnStmt{}
	if v, ok := raw["value"]; ok && v != nil {
		val, err := parseChildNode(raw, "value", depth...)
		if err != nil {
			return nil, err
		}
		rs.Value = val
	}
	return rs, nil
}

func parseVarDecl(raw map[string]interface{}, depth ...int) (*VarDecl, error) {
	vd := &VarDecl{}
	vd.Name, _ = raw["name"].(string)
	if vd.Name == "" {
		return nil, fmt.Errorf("XQL_E101: VarDecl missing 'name'")
	}
	if t, ok := raw["type"]; ok {
		te, err := parseTypeExpr(t, depth...)
		if err != nil {
			return nil, err
		}
		vd.Type = te
	}
	if v, ok := raw["value"]; ok && v != nil {
		val, err := parseChildNode(raw, "value", depth...)
		if err != nil {
			return nil, err
		}
		vd.Value = val
	}
	return vd, nil
}

func parseAssignStmt(raw map[string]interface{}, depth ...int) (*AssignStmt, error) {
	as := &AssignStmt{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	switch t := raw["target"].(type) {
	case string:
		if t == "" {
			return nil, fmt.Errorf("XQL_E101: AssignStmt missing 'target'")
		}
		as.Target = &Ident{Name: t}
	case map[string]interface{}:
		node, err := parseNode(t, curDepth+1)
		if err != nil {
			return nil, err
		}
		as.Target = node
	default:
		return nil, fmt.Errorf("XQL_E101: AssignStmt missing 'target'")
	}
	val, err := parseChildNode(raw, "value", depth...)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, fmt.Errorf("XQL_E101: AssignStmt missing 'value'")
	}
	as.Value = val
	return as, nil
}

func parseIfStmt(raw map[string]interface{}, depth ...int) (*IfStmt, error) {
	is := &IfStmt{}
	cond, err := parseChildNode(raw, "cond", depth...)
	if err != nil {
		return nil, err
	}
	if cond == nil {
		cond, err = parseChildNode(raw, "condition", depth...)
		if err != nil {
			return nil, err
		}
	}
	if cond == nil {
		return nil, fmt.Errorf("XQL_E101: IfStmt missing 'cond'")
	}
	is.Cond = cond

	if then, ok := raw["then"].([]interface{}); ok {
		nodes, err := parseNodeList(then, depth...)
		if err != nil {
			return nil, err
		}
		is.Then = nodes
	}
	if els, ok := raw["else"].([]interface{}); ok {
		nodes, err := parseNodeList(els, depth...)
		if err != nil {
			return nil, err
		}
		is.Else = nodes
	}
	return is, nil
}

func parseWhileStmt(raw map[string]interface{}, depth ...int) (*WhileStmt, error) {
	ws := &WhileStmt{}
	cond, err := parseChildNode(raw, "cond", depth...)
	if err != nil {
		return nil, err
	}
	if cond == nil {
		cond, err = parseChildNode(raw, "condition", depth...)
		if err != nil {
			return nil, err
		}
	}
	if cond == nil {
		return nil, fmt.Errorf("XQL_E101: WhileStmt missing 'cond'")
	}
	ws.Cond = cond
	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body, depth...)
		if err != nil {
			return nil, err
		}
		ws.Body = nodes
	}
	return ws, nil
}

func parseForStmt(raw map[string]interface{}, depth ...int) (*ForStmt, error) {
	fs := &ForStmt{}
	fs.Form, _ = raw["form"].(string)
	if fs.Form != "range" && fs.Form != "each" {
		return nil, fmt.Errorf("XQL_E101: ForStmt 'form' must be \"range\" or \"each\", got %q", fs.Form)
	}
	fs.Var, _ = raw["var"].(string)
	if fs.Var == "" {
		return nil, fmt.Errorf("XQL_E101: ForStmt missing 'var'")
	}
	if fs.Form == "range" {
		start, err := parseChildNode(raw, "start", depth...)
		if err != nil {
			return nil, err
		}
		if start == nil {
			return nil, fmt.Errorf("XQL_E101: ForStmt range form missing 'start'")
		}
		fs.Start = start
		end, err := parseChildNode(raw, "end", depth...)
		if err != nil {
			return nil, err
		}
		if end == nil {
			return nil, fmt.Errorf("XQL_E101: ForStmt range form missing 'end'")
		}
		fs.End = end
	} else {
		iter, err := parseChildNode(raw, "iterable", depth...)
		if err != nil {
			return nil, err
		}
		if iter == nil {
			return nil, fmt.Errorf("XQL_E101: ForStmt each form missing 'iterable'")
		}
		fs.Iterable = iter
	}
	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body, depth...)
		if err != nil {
			return nil, err
		}
		fs.Body = nodes
	}
	return fs, nil
}

func parseExprStmt(raw map[string]interface{}, depth ...int) (*ExprStmt, error) {
	es := &ExprStmt{}
	expr, err := parseChildNode(raw, "expr", depth...)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, fmt.Errorf("XQL_E101: ExprStmt missing 'expr'")
	}
	es.Expr = expr
	return es, nil
}

func parseStructDecl(raw map[string]interface{}) (*StructDecl, error) {
	sd := &StructDecl{}
	sd.Name, _ = raw["name"].(string)
	if fields, ok := raw["fields"].([]interface{}); ok {
		for _, f := range fields {
			fm, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			sf := StructField{}
			sf.Name, _ = fm["name"].(string)
			if t, ok := fm["type"]; ok {
				te, err := parseTypeExpr(t)
				if err != nil {
					return nil, err
				}
				sf.Type = te
			}
			sf.Visibility, _ = fm["visibility"].(string)
			sd.Fields = append(sd.Fields, sf)
		}
	}
	return sd, nil
}

func parseClassDecl(raw map[string]interface{}) (*ClassDecl, error) {
	cd := &ClassDecl{}
	cd.Name, _ = raw["name"].(string)
	if fields, ok := raw["fields"].([]interface{}); ok {
		for _, f := range fields {
			fm, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			cf := ClassField{}
			cf.Name, _ = fm["name"].(string)
			if t, ok := fm["type"]; ok {
				te, err := parseTypeExpr(t)
				if err != nil {
					return nil, err
				}
				cf.Type = te
			}
			cf.Visibility, _ = fm["visibility"].(string)
			cd.Fields = append(cd.Fields, cf)
		}
	}
	return cd, nil
}

func parseEnumDecl(raw map[string]interface{}) (*EnumDecl, error) {
	ed := &EnumDecl{}
	ed.Name, _ = raw["name"].(string)
	ed.Variants = parseStringList(raw["variants"])
	return ed, nil
}

func parseMatchExpr(raw map[string]interface{}, depth ...int) (*MatchExpr, error) {
	me := &MatchExpr{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	val, err := parseChildNode(raw, "value", depth...)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, fmt.Errorf("XQL_E101: MatchExpr missing 'value'")
	}
	me.Value = val
	if arms, ok := raw["arms"].([]interface{}); ok {
		for _, a := range arms {
			am, ok := a.(map[string]interface{})
			if !ok {
				continue
			}
			arm := MatchArm{}
			if p, ok := am["pattern"]; ok && p != nil {
				pattern, err := parseNode(p.(map[string]interface{}), curDepth+1)
				if err != nil {
					return nil, err
				}
				arm.Pattern = pattern
			}
			if bodyArr, ok := am["body"].([]interface{}); ok {
				nodes, err := parseNodeList(bodyArr, depth...)
				if err != nil {
					return nil, err
				}
				arm.Body = nodes
			}
			me.Arms = append(me.Arms, arm)
		}
	}
	return me, nil
}

func parseSwitchStmt(raw map[string]interface{}, depth ...int) (*SwitchStmt, error) {
	ss := &SwitchStmt{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	val, err := parseChildNode(raw, "value", depth...)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, fmt.Errorf("XQL_E101: SwitchStmt missing 'value'")
	}
	ss.Value = val
	if cases, ok := raw["cases"].([]interface{}); ok {
		for _, c := range cases {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			sc := SwitchCase{}
			if v, ok := cm["value"]; ok && v != nil {
				caseVal, err := parseNode(v.(map[string]interface{}), curDepth+1)
				if err != nil {
					return nil, err
				}
				sc.Value = caseVal
			}
			if bodyArr, ok := cm["body"].([]interface{}); ok {
				nodes, err := parseNodeList(bodyArr, depth...)
				if err != nil {
					return nil, err
				}
				sc.Body = nodes
			}
			ss.Cases = append(ss.Cases, sc)
		}
	}
	return ss, nil
}

func parseStructLit(raw map[string]interface{}, depth ...int) (*StructLit, error) {
	sl := &StructLit{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	sl.TypeName, _ = raw["typeName"].(string)
	if fields, ok := raw["fields"].([]interface{}); ok {
		for _, f := range fields {
			fm, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			sfi := StructFieldInit{}
			sfi.Name, _ = fm["name"].(string)
			if v, ok := fm["value"]; ok && v != nil {
				val, err := parseNode(v.(map[string]interface{}), curDepth+1)
				if err != nil {
					return nil, err
				}
				sfi.Value = val
			}
			sl.Fields = append(sl.Fields, sfi)
		}
	}
	return sl, nil
}

func parseArrayLit(raw map[string]interface{}, depth ...int) (*ArrayLit, error) {
	al := &ArrayLit{}
	if t, ok := raw["elemType"]; ok {
		te, err := parseTypeExpr(t, depth...)
		if err != nil {
			return nil, err
		}
		al.ElemType = te
	}
	if elems, ok := raw["elements"].([]interface{}); ok {
		nodes, err := parseNodeList(elems, depth...)
		if err != nil {
			return nil, err
		}
		al.Elements = nodes
	}
	return al, nil
}

func parseMapLiteral(raw map[string]interface{}, depth ...int) (*MapLiteral, error) {
	ml := &MapLiteral{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	if t, ok := raw["keyType"]; ok {
		te, err := parseTypeExpr(t, depth...)
		if err != nil {
			return nil, err
		}
		ml.KeyType = te
	}
	if t, ok := raw["valueType"]; ok {
		te, err := parseTypeExpr(t, depth...)
		if err != nil {
			return nil, err
		}
		ml.ValueType = te
	}
	if entries, ok := raw["entries"].([]interface{}); ok {
		for _, e := range entries {
			em, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			entry := MapEntry{}
			if k, ok := em["key"]; ok && k != nil {
				keyNode, err := parseNode(k.(map[string]interface{}), curDepth+1)
				if err != nil {
					return nil, err
				}
				entry.Key = keyNode
			}
			if v, ok := em["value"]; ok && v != nil {
				valNode, err := parseNode(v.(map[string]interface{}), curDepth+1)
				if err != nil {
					return nil, err
				}
				entry.Value = valNode
			}
			ml.Entries = append(ml.Entries, entry)
		}
	}
	return ml, nil
}

func parseIndexExpr(raw map[string]interface{}, depth ...int) (*IndexExpr, error) {
	ie := &IndexExpr{}
	target, err := parseChildNode(raw, "target", depth...)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("XQL_E101: IndexExpr missing 'target'")
	}
	ie.Target = target
	index, err := parseChildNode(raw, "index", depth...)
	if err != nil {
		return nil, err
	}
	if index == nil {
		return nil, fmt.Errorf("XQL_E101: IndexExpr missing 'index'")
	}
	ie.Index = index
	return ie, nil
}

func parseIfExpr(raw map[string]interface{}, depth ...int) (*IfExpr, error) {
	ie := &IfExpr{}
	cond, err := parseChildNode(raw, "cond", depth...)
	if err != nil {
		return nil, err
	}
	if cond == nil {
		cond, err = parseChildNode(raw, "condition", depth...)
		if err != nil {
			return nil, err
		}
	}
	if cond == nil {
		return nil, fmt.Errorf("XQL_E101: IfExpr missing 'cond'")
	}
	ie.Cond = cond
	thenNode, err := parseChildNode(raw, "then", depth...)
	if err != nil {
		return nil, err
	}
	if thenNode == nil {
		return nil, fmt.Errorf("XQL_E101: IfExpr missing 'then'")
	}
	ie.Then = thenNode
	elseNode, err := parseChildNode(raw, "else", depth...)
	if err != nil {
		return nil, err
	}
	if elseNode == nil {
		return nil, fmt.Errorf("XQL_E101: IfExpr missing 'else'")
	}
	ie.Else = elseNode
	return ie, nil
}

func parseLambda(raw map[string]interface{}, depth ...int) (*Lambda, error) {
	l := &Lambda{}
	if params, ok := raw["params"].([]interface{}); ok {
		for _, p := range params {
			pm, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			param := Param{}
			param.Name, _ = pm["name"].(string)
			if t, ok := pm["type"]; ok {
				te, err := parseTypeExpr(t, depth...)
				if err != nil {
					return nil, err
				}
				param.Type = te
			}
			l.Params = append(l.Params, param)
		}
	}
	if rt, ok := raw["returnType"]; ok {
		te, err := parseTypeExpr(rt, depth...)
		if err != nil {
			return nil, err
		}
		l.ReturnType = te
	}
	if bodyArr, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(bodyArr, depth...)
		if err != nil {
			return nil, err
		}
		l.Body = nodes
	}
	return l, nil
}

func parseBinaryExpr(raw map[string]interface{}, depth ...int) (*BinaryExpr, error) {
	be := &BinaryExpr{}
	be.Op, _ = raw["op"].(string)
	left, err := parseChildNode(raw, "left", depth...)
	if err != nil {
		return nil, err
	}
	if left == nil {
		return nil, fmt.Errorf("XQL_E101: BinaryExpr missing 'left'")
	}
	be.Left = left
	right, err := parseChildNode(raw, "right", depth...)
	if err != nil {
		return nil, err
	}
	if right == nil {
		return nil, fmt.Errorf("XQL_E101: BinaryExpr missing 'right'")
	}
	be.Right = right
	return be, nil
}

func parseUnaryExpr(raw map[string]interface{}, depth ...int) (*UnaryExpr, error) {
	ue := &UnaryExpr{}
	ue.Op, _ = raw["op"].(string)
	operand, err := parseChildNode(raw, "operand", depth...)
	if err != nil {
		return nil, err
	}
	if operand == nil {
		return nil, fmt.Errorf("XQL_E101: UnaryExpr missing 'operand'")
	}
	ue.Operand = operand
	return ue, nil
}

func parseCallExpr(raw map[string]interface{}, depth ...int) (*CallExpr, error) {
	ce := &CallExpr{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	ce.Callee, _ = raw["callee"].(string)
	if args, ok := raw["args"].([]interface{}); ok {
		for _, a := range args {
			am, ok := a.(map[string]interface{})
			if !ok {
				continue
			}
			arg, err := parseNode(am, curDepth+1)
			if err != nil {
				return nil, err
			}
			ce.Args = append(ce.Args, arg)
		}
	}
	return ce, nil
}

func parseLiteral(raw map[string]interface{}) (*Literal, error) {
	l := &Literal{}
	l.ValueType, _ = raw["valueType"].(string)
	l.Value = raw["value"]
	return l, nil
}

func parseIdent(raw map[string]interface{}) (*Ident, error) {
	i := &Ident{}
	i.Name, _ = raw["name"].(string)
	return i, nil
}

func parseMemberExpr(raw map[string]interface{}, depth ...int) (*MemberExpr, error) {
	me := &MemberExpr{}
	me.Field, _ = raw["field"].(string)
	obj, err := parseChildNode(raw, "object", depth...)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("XQL_E101: MemberExpr missing 'object'")
	}
	me.Object = obj
	return me, nil
}

func parseNewExpr(raw map[string]interface{}, depth ...int) (*NewExpr, error) {
	ne := &NewExpr{}
	curDepth := 1
	if len(depth) > 0 {
		curDepth = depth[0]
	}
	ne.Callee, _ = raw["callee"].(string)
	if args, ok := raw["args"].([]interface{}); ok {
		for _, a := range args {
			am, ok := a.(map[string]interface{})
			if !ok {
				continue
			}
			arg, err := parseNode(am, curDepth+1)
			if err != nil {
				return nil, err
			}
			ne.Args = append(ne.Args, arg)
		}
	}
	return ne, nil
}

func parseAwaitExpr(raw map[string]interface{}, depth ...int) (*AwaitExpr, error) {
	ae := &AwaitExpr{}
	expr, err := parseChildNode(raw, "expr", depth...)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, fmt.Errorf("XQL_E101: AwaitExpr missing 'expr'")
	}
	ae.Expr = expr
	return ae, nil
}
