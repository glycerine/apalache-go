package smtcore

// Source: src/smt/theory_bv.cpp internalize_carry/internalize_xor3.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTBitVectorTheory) internalizeBooleanGate3(expression ast.ASTHandle, operation int32, gateContext bool) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	n, err := s.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	if n != 3 {
		return false, ast.ErrASTDomain
	}
	var args [3]SMTLiteral
	for i := range args {
		h, e := s.manager.GetArg(expression, uint32(i))
		if e != nil {
			return false, e
		}
		if e = s.Internalize(h, true); e != nil {
			return false, e
		}
		args[i], e = s.Literal(h)
		if e != nil {
			return false, e
		}
	}
	variable, exists := s.BoolVar(expression)
	if !exists {
		variable, err = s.MkBoolVar(expression)
		if err != nil {
			return false, err
		}
		r := NewSMTLiteral(variable, false)
		a, b, c := args[0], args[1], args[2]
		var clauses [][]SMTLiteral
		if operation == 59 {
			clauses = [][]SMTLiteral{{r.Negated(), a, b}, {r.Negated(), a, c}, {r.Negated(), b, c}, {r, a.Negated(), b.Negated()}, {r, a.Negated(), c.Negated()}, {r, b.Negated(), c.Negated()}}
		} else {
			clauses = [][]SMTLiteral{{r.Negated(), a, b, c}, {r.Negated(), a.Negated(), b.Negated(), c}, {r.Negated(), a.Negated(), b, c.Negated()}, {r.Negated(), a, b.Negated(), c.Negated()}, {r, a.Negated(), b, c}, {r, a, b.Negated(), c}, {r, a, b, c.Negated()}, {r, a.Negated(), b.Negated(), c.Negated()}}
		}
		for _, clause := range clauses {
			if err = s.addGateClause(clause); err != nil {
				return false, err
			}
		}
	}
	if !gateContext && s.context.enodes[expression] == nil {
		if err = s.ensureBooleanEnode(expression, variable, true, true, true); err != nil {
			return false, err
		}
	}
	return true, nil
}
