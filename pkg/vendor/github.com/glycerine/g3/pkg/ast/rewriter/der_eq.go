package rewriter

// Source: src/ast/rewriter/der.cpp:98-132,166-218.

import "github.com/glycerine/g3/pkg/ast"

// isVarEq recognizes the existential duals of isVarDiseq.
func (d *DER) isVarEq(h ast.ASTHandle, declarations uint32) (variable ast.ASTHandle, index uint32, definition ast.ASTHandle, matched bool, err error) {
	defer func() {
		if matched && err == nil {
			err = d.traceRecognized("is_var_eq", 100, h)
			if err != nil {
				matched = false
			}
		}
	}()
	if err = d.open(); err != nil {
		return
	}
	isEq, err := d.isBasic(h, ast.ASTOpEq)
	if err != nil {
		return variable, index, definition, false, err
	}
	if isEq {
		lhs, x := d.manager.GetArg(h, 0)
		if x != nil {
			return variable, index, definition, false, x
		}
		rhs, x := d.manager.GetArg(h, 1)
		if x != nil {
			return variable, index, definition, false, x
		}
		idx, left, x := d.boundVariable(lhs, declarations)
		if x != nil {
			return variable, index, definition, false, x
		}
		if !left {
			idx, left, x = d.boundVariable(rhs, declarations)
			if x != nil {
				return variable, index, definition, false, x
			}
			if !left {
				return variable, index, definition, false, nil
			}
			lhs, rhs = rhs, lhs
		}
		return lhs, idx, rhs, true, nil
	}
	idx, isVar, err := d.boundVariable(h, declarations)
	if err != nil {
		return variable, index, definition, false, err
	}
	if isVar {
		value, x := d.manager.MkTrue()
		return h, idx, value, x == nil, x
	}
	variable, idx, negated, err := d.negatedBoundVariable(h, declarations)
	if err != nil || !negated {
		return ast.ASTHandle{}, 0, ast.ASTHandle{}, false, err
	}
	value, err := d.manager.MkFalse()
	return variable, idx, value, err == nil, err
}

func (d *DER) matchVariableLiteral(c *derCall, h ast.ASTHandle, declarations uint32, forall bool) (ast.ASTHandle, uint32, ast.ASTHandle, bool, error) {
	if forall {
		return d.isVarDiseq(c, h, declarations)
	}
	return d.isVarEq(h, declarations)
}
