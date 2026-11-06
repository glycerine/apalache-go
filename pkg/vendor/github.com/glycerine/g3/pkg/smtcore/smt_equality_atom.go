package smtcore

// Source: src/smt/smt_context.cpp context::mk_eq_atom, src/smt/smt_theory.h
// theory::mk_eq_atom, and src/ast/arith_decl_plugin.h arith_util::mk_eq.

import "github.com/glycerine/g3/pkg/ast"

// mkEqualityAtom uses the operand sort's theory convention for atoms created
// during search. Arithmetic puts numerals on the right; other theories order
// operands by AST ID. User assertions retain their original syntax.
func (s *SMTInternalizerState) mkEqualityAtom(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.manager
	sort, err := m.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftID, err := m.GetASTID(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightID, err := m.GetASTID(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch s.theories[family].(type) {
	case *SMTLinearArithmeticTheory, *SMTLRATheory, *SMTRealDiffTheory, *SMTDenseDiffTheory, *SMTUTVPITheory:
		_, _, leftNumeral, err := m.ArithmeticNumeralText(left)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		_, _, rightNumeral, err := m.ArithmeticNumeralText(right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if leftNumeral || (!rightNumeral && leftID > rightID) {
			left, right = right, left
		}
		if left == right {
			return m.MkTrue()
		}
		if leftNumeral && rightNumeral {
			return m.MkFalse()
		}
		return m.MkEq(left, right)
	default:
		if leftID > rightID {
			left, right = right, left
		}
		return m.MkEq(left, right)
	}
}
