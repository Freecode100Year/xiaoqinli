package check

import (
	"fmt"
	"reflect"
	"regexp"

	"xiaoqinli/ast"
)

// Every backend writes names and operators into its output verbatim, so a name
// is source code. A VarDecl named "x = __import__('os').system('id')\n    y"
// used to pass every check — the program was pure and held no grants — and the
// Python it compiled to ran a shell command. Whatever a capability check
// proves about a program is only true if the text the backends emit is the
// program that was checked, so names and operators are held to a grammar
// before anything else looks at them.
var (
	identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// A reference to a host symbol: `document.body.appendChild`,
	// `time.Now().Format`.
	refRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\(\))?(\.[A-Za-z_][A-Za-z0-9_]*(\(\))?)*$`)
	// A type from an imported module: `models.Config`.
	qualRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	pathRe = regexp.MustCompile(`^[A-Za-z0-9_./@+-]+$`)
	// Capability and effect names: `io`, `browser:tabs`, `network:*`.
	capRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*([:.\-]([A-Za-z0-9_]+|\*))*$`)

	binaryOps = map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true,
		"==": true, "!=": true, "===": true, "!==": true, "<": true, ">": true, "<=": true, ">=": true,
		"&&": true, "||": true}
	unaryOps = map[string]bool{"!": true, "-": true}
)

// CheckIdentifiers reports every name, operator and tag in the tree that a
// backend could not emit as exactly that token.
func CheckIdentifiers(root ast.Node) []string {
	var errs []string
	walkIdents(reflect.ValueOf(root), "", &errs)
	return errs
}

func walkIdents(v reflect.Value, owner string, errs *[]string) {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if !v.IsNil() {
			walkIdents(v.Elem(), owner, errs)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walkIdents(v.Index(i), owner, errs)
		}
	case reflect.Struct:
		if lit, ok := v.Interface().(ast.Literal); ok {
			checkLiteral(&lit, errs)
		}
		t := v.Type()
		for _, name := range requiredChildren(v) {
			if f := v.FieldByName(name); f.IsValid() && f.IsNil() {
				*errs = append(*errs, fmt.Sprintf("invalid %s: %s is missing", t.Name(), name))
			}
		}
		for i := 0; i < t.NumField(); i++ {
			f, fv := t.Field(i), v.Field(i)
			if !f.IsExported() {
				continue
			}
			if fv.Kind() == reflect.String {
				checkIdentField(t.Name(), f.Name, fv.String(), errs)
			} else if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.String {
				for j := 0; j < fv.Len(); j++ {
					checkIdentField(t.Name(), f.Name, fv.Index(j).String(), errs)
				}
			} else {
				walkIdents(fv, t.Name(), errs)
			}
		}
	}
}

func checkIdentField(typ, field, s string, errs *[]string) {
	bad := func(what string) {
		*errs = append(*errs, fmt.Sprintf("invalid %s %q in %s.%s", what, s, typ, field))
	}
	switch field {
	case "Name", "Callee":
		if typ == "Ident" || typ == "CallExpr" || typ == "NewExpr" || typ == "ExternDecl" {
			if !refRe.MatchString(s) {
				bad("identifier")
			}
		} else if !identRe.MatchString(s) {
			bad("identifier")
		}
	case "TypeName":
		if !qualRe.MatchString(s) {
			bad("identifier")
		}
	case "Field", "Var", "Variants", "Targets", "ValueType":
		if !identRe.MatchString(s) {
			bad("identifier")
		}
	case "KindName":
		if s != "" && !qualRe.MatchString(s) {
			bad("identifier")
		}
	case "Form":
		if s != "range" && s != "each" {
			bad("for-loop form")
		}
	case "As":
		if s != "" && !identRe.MatchString(s) {
			bad("identifier")
		}
	case "Effects", "Grant":
		if !capRe.MatchString(s) {
			bad("capability name")
		}
	case "Visibility":
		if s != "" && s != "public" && s != "private" {
			bad("visibility")
		}
	case "Op":
		if (typ == "UnaryExpr" && !unaryOps[s]) || (typ != "UnaryExpr" && !binaryOps[s]) {
			bad("operator")
		}
	case "Path":
		// Backends write the path into a quoted string, and several of
		// those strings interpolate (Julia's include, Ruby's require_relative).
		if !pathRe.MatchString(s) {
			bad("import path")
		}
	}
}

// requiredChildren names the child nodes a node is meaningless without. The
// backends index into them unguarded, so a match arm with no pattern was a
// nil dereference in the compiler rather than an error in the program.
func requiredChildren(v reflect.Value) []string {
	switch v.Type().Name() {
	case "MatchArm":
		return []string{"Pattern"}
	case "MatchExpr", "SwitchStmt", "StructFieldInit":
		return []string{"Value"}
	case "BinaryExpr":
		return []string{"Left", "Right"}
	case "UnaryExpr":
		return []string{"Operand"}
	case "MemberExpr":
		return []string{"Object"}
	case "IndexExpr":
		return []string{"Target", "Index"}
	case "IfExpr":
		return []string{"Cond", "Then", "Else"}
	case "IfStmt", "WhileStmt":
		return []string{"Cond"}
	case "AssignStmt":
		return []string{"Target", "Value"}
	case "ExprStmt", "AwaitExpr":
		return []string{"Expr"}
	case "MapEntry":
		return []string{"Key", "Value"}
	case "ForStmt":
		if v.FieldByName("Form").String() == "each" {
			return []string{"Iterable"}
		}
		return []string{"Start", "End"}
	}
	return nil
}

// checkLiteral rejects a literal whose value is not the type it claims. The
// backends switch on ValueType and fall back to writing the value with %v, so
// {"valueType":"Int","value":"0; rm -rf ~"} reached the output unquoted.
func checkLiteral(lit *ast.Literal, errs *[]string) {
	ok := false
	switch lit.ValueType {
	case "String":
		_, ok = lit.Value.(string)
	case "Int", "Float":
		switch lit.Value.(type) {
		case float64, int64, int:
			ok = true
		}
	case "Bool":
		_, ok = lit.Value.(bool)
	}
	if !ok {
		*errs = append(*errs, fmt.Sprintf("invalid literal: value %v (%T) is not a %s", lit.Value, lit.Value, lit.ValueType))
	}
}
