package ast

import (
	"encoding/json"
	"fmt"
)

// Node is the interface all AST nodes implement.
type Node interface {
	Kind() string
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
	Name string   `json:"name"`
	Type TypeExpr `json:"type"`
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

// Lambda represents an anonymous function / closure expression.
type Lambda struct {
	Params     []Param
	ReturnType TypeExpr
	Body       []Node
}

func (*Lambda) Kind() string { return "Lambda" }

// ===================== JSON Parsing =====================

// Parse parses .xql.json bytes into a typed AST tree.
func Parse(data []byte) (Node, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("XQL_E101: invalid JSON: %w", err)
	}
	return parseNode(raw)
}

func parseNode(raw map[string]interface{}) (Node, error) {
	kind, ok := raw["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("XQL_E101: node missing 'kind' field")
	}
	switch kind {
	case "Program":
		return parseProgram(raw)
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
	case "EnumDecl":
		return parseEnumDecl(raw)
	case "MatchExpr":
		return parseMatchExpr(raw)
	case "StructLit":
		return parseStructLit(raw)
	case "ArrayLit":
		return parseArrayLit(raw)
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
	if !ok {
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

// --- Individual node parsers ---

func parseProgram(raw map[string]interface{}) (*Program, error) {
	decls, ok := raw["declarations"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("XQL_E101: Program missing 'declarations'")
	}
	nodes, err := parseNodeList(decls)
	if err != nil {
		return nil, err
	}
	return &Program{Decls: nodes}, nil
}

func parseFunctionDecl(raw map[string]interface{}) (*FunctionDecl, error) {
	fd := &FunctionDecl{}
	fd.Name, _ = raw["name"].(string)
	if fd.Name == "" {
		return nil, fmt.Errorf("XQL_E101: FunctionDecl missing 'name'")
	}

	if params, ok := raw["params"].([]interface{}); ok {
		for _, p := range params {
			pm, ok := p.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("XQL_E101: param is not an object")
			}
			param := Param{}
			param.Name, _ = pm["name"].(string)
			if t, ok := pm["type"]; ok {
				te, err := parseTypeExpr(t)
				if err != nil {
					return nil, err
				}
				param.Type = te
			}
			fd.Params = append(fd.Params, param)
		}
	}

	if rt, ok := raw["returnType"]; ok {
		te, err := parseTypeExpr(rt)
		if err != nil {
			return nil, err
		}
		fd.ReturnType = te
	}

	fd.Effects = parseStringList(raw["effects"])
	fd.Grant = parseStringList(raw["grant"])

	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body)
		if err != nil {
			return nil, err
		}
		fd.Body = nodes
	}

	return fd, nil
}

func parseReturnStmt(raw map[string]interface{}) (*ReturnStmt, error) {
	val, err := parseChildNode(raw, "value")
	if err != nil {
		return nil, err
	}
	return &ReturnStmt{Value: val}, nil
}

func parseVarDecl(raw map[string]interface{}) (*VarDecl, error) {
	vd := &VarDecl{}
	vd.Name, _ = raw["name"].(string)
	if vd.Name == "" {
		return nil, fmt.Errorf("XQL_E101: VarDecl missing 'name'")
	}
	if t, ok := raw["type"]; ok {
		te, err := parseTypeExpr(t)
		if err != nil {
			return nil, err
		}
		vd.Type = te
	}
	val, err := parseChildNode(raw, "value")
	if err != nil {
		return nil, err
	}
	vd.Value = val
	return vd, nil
}

func parseAssignStmt(raw map[string]interface{}) (*AssignStmt, error) {
	as := &AssignStmt{}
	switch t := raw["target"].(type) {
	case string:
		if t == "" {
			return nil, fmt.Errorf("XQL_E101: AssignStmt missing 'target'")
		}
		as.Target = &Ident{Name: t}
	case map[string]interface{}:
		node, err := parseNode(t)
		if err != nil {
			return nil, err
		}
		as.Target = node
	default:
		return nil, fmt.Errorf("XQL_E101: AssignStmt missing 'target'")
	}
	val, err := parseChildNode(raw, "value")
	if err != nil {
		return nil, err
	}
	as.Value = val
	return as, nil
}

func parseIfStmt(raw map[string]interface{}) (*IfStmt, error) {
	is := &IfStmt{}
	cond, err := parseChildNode(raw, "cond")
	if err != nil {
		return nil, err
	}
	if cond == nil {
		cond, err = parseChildNode(raw, "condition")
		if err != nil {
			return nil, err
		}
	}
	if cond == nil {
		return nil, fmt.Errorf("XQL_E101: IfStmt missing 'cond'")
	}
	is.Cond = cond

	if then, ok := raw["then"].([]interface{}); ok {
		nodes, err := parseNodeList(then)
		if err != nil {
			return nil, err
		}
		is.Then = nodes
	}
	if els, ok := raw["else"].([]interface{}); ok {
		nodes, err := parseNodeList(els)
		if err != nil {
			return nil, err
		}
		is.Else = nodes
	}
	return is, nil
}

func parseWhileStmt(raw map[string]interface{}) (*WhileStmt, error) {
	ws := &WhileStmt{}
	cond, err := parseChildNode(raw, "cond")
	if err != nil {
		return nil, err
	}
	if cond == nil {
		cond, err = parseChildNode(raw, "condition")
		if err != nil {
			return nil, err
		}
	}
	if cond == nil {
		return nil, fmt.Errorf("XQL_E101: WhileStmt missing 'cond'")
	}
	ws.Cond = cond

	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body)
		if err != nil {
			return nil, err
		}
		ws.Body = nodes
	}
	return ws, nil
}

func parseForStmt(raw map[string]interface{}) (*ForStmt, error) {
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
		start, err := parseChildNode(raw, "start")
		if err != nil {
			return nil, err
		}
		if start == nil {
			return nil, fmt.Errorf("XQL_E101: ForStmt range form missing 'start'")
		}
		fs.Start = start
		end, err := parseChildNode(raw, "end")
		if err != nil {
			return nil, err
		}
		if end == nil {
			return nil, fmt.Errorf("XQL_E101: ForStmt range form missing 'end'")
		}
		fs.End = end
	} else {
		iter, err := parseChildNode(raw, "iterable")
		if err != nil {
			return nil, err
		}
		if iter == nil {
			return nil, fmt.Errorf("XQL_E101: ForStmt each form missing 'iterable'")
		}
		fs.Iterable = iter
	}
	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body)
		if err != nil {
			return nil, err
		}
		fs.Body = nodes
	}
	return fs, nil
}

func parseExprStmt(raw map[string]interface{}) (*ExprStmt, error) {
	expr, err := parseChildNode(raw, "expr")
	if err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, fmt.Errorf("XQL_E101: ExprStmt missing 'expr'")
	}
	return &ExprStmt{Expr: expr}, nil
}

func parseBinaryExpr(raw map[string]interface{}) (*BinaryExpr, error) {
	be := &BinaryExpr{}
	be.Op, _ = raw["op"].(string)
	if be.Op == "" {
		return nil, fmt.Errorf("XQL_E101: BinaryExpr missing 'op'")
	}
	left, err := parseChildNode(raw, "left")
	if err != nil {
		return nil, err
	}
	be.Left = left
	right, err := parseChildNode(raw, "right")
	if err != nil {
		return nil, err
	}
	be.Right = right
	return be, nil
}

func parseUnaryExpr(raw map[string]interface{}) (*UnaryExpr, error) {
	ue := &UnaryExpr{}
	ue.Op, _ = raw["op"].(string)
	operand, err := parseChildNode(raw, "operand")
	if err != nil {
		return nil, err
	}
	ue.Operand = operand
	return ue, nil
}

func parseCallExpr(raw map[string]interface{}) (*CallExpr, error) {
	ce := &CallExpr{}
	ce.Callee, _ = raw["callee"].(string)
	if ce.Callee == "" {
		return nil, fmt.Errorf("XQL_E101: CallExpr missing 'callee'")
	}
	if args, ok := raw["args"].([]interface{}); ok {
		nodes, err := parseNodeList(args)
		if err != nil {
			return nil, err
		}
		ce.Args = nodes
	}
	return ce, nil
}

func parseLiteral(raw map[string]interface{}) (*Literal, error) {
	lit := &Literal{}
	lit.ValueType, _ = raw["valueType"].(string)
	if lit.ValueType == "" {
		return nil, fmt.Errorf("XQL_E101: Literal missing 'valueType'")
	}
	lit.Value = raw["value"]
	return lit, nil
}

func parseIdent(raw map[string]interface{}) (*Ident, error) {
	name, _ := raw["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("XQL_E101: Ident missing 'name'")
	}
	return &Ident{Name: name}, nil
}

func parseMemberExpr(raw map[string]interface{}) (*MemberExpr, error) {
	me := &MemberExpr{}
	me.Field, _ = raw["field"].(string)
	if me.Field == "" {
		return nil, fmt.Errorf("XQL_E101: MemberExpr missing 'field'")
	}
	obj, err := parseChildNode(raw, "object")
	if err != nil {
		return nil, err
	}
	me.Object = obj
	return me, nil
}

func parseStructDecl(raw map[string]interface{}) (*StructDecl, error) {
	sd := &StructDecl{}
	sd.Name, _ = raw["name"].(string)
	if sd.Name == "" {
		return nil, fmt.Errorf("XQL_E101: StructDecl missing 'name'")
	}
	if fields, ok := raw["fields"].([]interface{}); ok {
		for _, f := range fields {
			fm, ok := f.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("XQL_E101: StructDecl field is not an object")
			}
			sf := StructField{}
			sf.Name, _ = fm["name"].(string)
			if sf.Name == "" {
				return nil, fmt.Errorf("XQL_E101: StructDecl field missing 'name'")
			}
			if t, ok := fm["type"]; ok {
				te, err := parseTypeExpr(t)
				if err != nil {
					return nil, err
				}
				sf.Type = te
			}
			sd.Fields = append(sd.Fields, sf)
		}
	}
	return sd, nil
}

func parseStructLit(raw map[string]interface{}) (*StructLit, error) {
	sl := &StructLit{}
	sl.TypeName, _ = raw["typeName"].(string)
	if sl.TypeName == "" {
		return nil, fmt.Errorf("XQL_E101: StructLit missing 'typeName'")
	}
	if fields, ok := raw["fields"].([]interface{}); ok {
		for _, f := range fields {
			fm, ok := f.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("XQL_E101: StructLit field is not an object")
			}
			fi := StructFieldInit{}
			fi.Name, _ = fm["name"].(string)
			if fi.Name == "" {
				return nil, fmt.Errorf("XQL_E101: StructLit field missing 'name'")
			}
			val, err := parseChildNode(fm, "value")
			if err != nil {
				return nil, err
			}
			fi.Value = val
			sl.Fields = append(sl.Fields, fi)
		}
	}
	return sl, nil
}

func parseArrayLit(raw map[string]interface{}) (*ArrayLit, error) {
	al := &ArrayLit{}
	if t, ok := raw["elemType"]; ok {
		te, err := parseTypeExpr(t)
		if err != nil {
			return nil, err
		}
		al.ElemType = te
	}
	if elems, ok := raw["elements"].([]interface{}); ok {
		nodes, err := parseNodeList(elems)
		if err != nil {
			return nil, err
		}
		al.Elements = nodes
	}
	return al, nil
}

func parseEnumDecl(raw map[string]interface{}) (*EnumDecl, error) {
	ed := &EnumDecl{}
	ed.Name, _ = raw["name"].(string)
	if ed.Name == "" {
		return nil, fmt.Errorf("XQL_E101: EnumDecl missing 'name'")
	}
	ed.Variants = parseStringList(raw["variants"])
	if len(ed.Variants) == 0 {
		return nil, fmt.Errorf("XQL_E101: EnumDecl '%s' missing 'variants'", ed.Name)
	}
	return ed, nil
}

func parseMatchExpr(raw map[string]interface{}) (*MatchExpr, error) {
	me := &MatchExpr{}
	val, err := parseChildNode(raw, "value")
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, fmt.Errorf("XQL_E101: MatchExpr missing 'value'")
	}
	me.Value = val

	arms, ok := raw["arms"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("XQL_E101: MatchExpr missing 'arms'")
	}
	for _, a := range arms {
		am, ok := a.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("XQL_E101: MatchExpr arm is not an object")
		}
		arm := MatchArm{}
		pat, err := parseChildNode(am, "pattern")
		if err != nil {
			return nil, err
		}
		if pat == nil {
			return nil, fmt.Errorf("XQL_E101: MatchExpr arm missing 'pattern'")
		}
		arm.Pattern = pat
		if body, ok := am["body"].([]interface{}); ok {
			nodes, err := parseNodeList(body)
			if err != nil {
				return nil, err
			}
			arm.Body = nodes
		}
		me.Arms = append(me.Arms, arm)
	}
	return me, nil
}

func parseIndexExpr(raw map[string]interface{}) (*IndexExpr, error) {
	ie := &IndexExpr{}
	target, err := parseChildNode(raw, "target")
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("XQL_E101: IndexExpr missing 'target'")
	}
	ie.Target = target
	index, err := parseChildNode(raw, "index")
	if err != nil {
		return nil, err
	}
	if index == nil {
		return nil, fmt.Errorf("XQL_E101: IndexExpr missing 'index'")
	}
	ie.Index = index
	return ie, nil
}

func parseIfExpr(raw map[string]interface{}) (*IfExpr, error) {
	ie := &IfExpr{}
	cond, err := parseChildNode(raw, "cond")
	if err != nil {
		return nil, err
	}
	if cond == nil {
		cond, err = parseChildNode(raw, "condition")
		if err != nil {
			return nil, err
		}
	}
	if cond == nil {
		return nil, fmt.Errorf("XQL_E101: IfExpr missing 'cond'")
	}
	ie.Cond = cond
	thenN, err := parseChildNode(raw, "then")
	if err != nil {
		return nil, err
	}
	if thenN == nil {
		return nil, fmt.Errorf("XQL_E101: IfExpr missing 'then'")
	}
	ie.Then = thenN
	elseN, err := parseChildNode(raw, "else")
	if err != nil {
		return nil, err
	}
	if elseN == nil {
		return nil, fmt.Errorf("XQL_E101: IfExpr missing 'else'")
	}
	ie.Else = elseN
	return ie, nil
}

func parseLambda(raw map[string]interface{}) (*Lambda, error) {
	lam := &Lambda{}
	if params, ok := raw["params"].([]interface{}); ok {
		for _, p := range params {
			pm, ok := p.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("XQL_E101: Lambda param is not an object")
			}
			param := Param{}
			param.Name, _ = pm["name"].(string)
			if t, ok := pm["type"]; ok {
				te, err := parseTypeExpr(t)
				if err != nil {
					return nil, err
				}
				param.Type = te
			}
			lam.Params = append(lam.Params, param)
		}
	}
	if rt, ok := raw["returnType"]; ok {
		te, err := parseTypeExpr(rt)
		if err != nil {
			return nil, err
		}
		lam.ReturnType = te
	}
	if body, ok := raw["body"].([]interface{}); ok {
		nodes, err := parseNodeList(body)
		if err != nil {
			return nil, err
		}
		lam.Body = nodes
	}
	return lam, nil
}
