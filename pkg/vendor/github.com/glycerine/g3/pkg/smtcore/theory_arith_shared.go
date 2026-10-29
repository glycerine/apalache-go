package smtcore

// Source: src/smt/smt_context.cpp is_shared and
// src/smt/theory_arith_aux.h arithmetic is_shared override.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTLinearArithmeticTheory) sharedArithmeticNode(node *SMTEnode) (bool, error) {
	return smtSharedArithmeticNode(t.SMTTheoryBase, node, len(t.divisionTerms) != 0)
}

// Source context::is_shared common cases. consultTheory is true only for a
// single-theory class whose owner-specific sharing rule remains to be checked.
func (s *SMTInternalizerState) sharedTheoryNode(node *SMTEnode) (shared, consultTheory bool, result error) {
	if err := s.open(); err != nil {
		return false, false, err
	}
	if node == nil || node.manager != s.manager || node.root == nil {
		return false, false, ast.ErrASTDomain
	}
	root := node.root
	family, err := s.manager.GetFamilyID(root.owner)
	if err != nil {
		return false, false, err
	}
	if family == s.manager.BasicFamilyID() {
		kind, err := s.manager.GetDeclKind(root.owner)
		if err != nil {
			return false, false, err
		}
		if kind == ast.ASTOpIte {
			return true, false, nil
		}
	}
	count := root.NumTheoryVars()
	if count == 0 {
		return false, false, nil
	}
	if count > 1 {
		return true, false, nil
	}
	if q := s.quantifierManager; q != nil && q.IsShared(root) {
		return true, false, nil
	}
	theory := root.theoryVariables.theory
	for _, parent := range root.parents {
		family, err := s.manager.GetFamilyID(parent.owner)
		if err != nil {
			return false, false, err
		}
		if family != theory && family != s.manager.BasicFamilyID() {
			return true, false, nil
		}
	}
	return false, true, nil
}

func smtSharedArithmeticNode(owner *SMTTheoryBase, node *SMTEnode, division bool) (bool, error) {
	s, err := owner.State()
	if err != nil {
		return false, err
	}
	shared, consult, err := s.sharedTheoryNode(node)
	if err != nil || !consult || !division {
		return shared, err
	}
	for _, parent := range node.root.parents {
		family, err := s.manager.GetFamilyID(parent.owner)
		if err != nil {
			return false, err
		}
		if family == owner.FamilyID() {
			kind, err := s.manager.GetDeclKind(parent.owner)
			if err != nil {
				return false, err
			}
			if kind == smtArithOpDiv || kind == smtArithOpIDiv || kind == smtArithOpRem || kind == smtArithOpMod {
				return true, nil
			}
		}
	}
	return false, nil
}
