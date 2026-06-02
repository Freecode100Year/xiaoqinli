package check

import "xiaoqinli/ast"

// RunAll performs all static checks on the typed AST:
// 1. Type checking (types + return values + operator compatibility)
// 2. Effect inference and verification (@effects purity)
// 3. Capability verification (@grant inheritance)
//
// Any check failure returns an error; all checks must pass before codegen.
func RunAll(root ast.Node) error {
	// Check 1: Type checking.
	tc := NewTypeChecker()
	if err := tc.Check(root); err != nil {
		return err
	}

	// Check 2: Effect verification.
	if err := CheckEffects(root); err != nil {
		return err
	}

	// Check 3: Capability verification.
	if err := CheckCapabilities(root); err != nil {
		return err
	}

	return nil
}
