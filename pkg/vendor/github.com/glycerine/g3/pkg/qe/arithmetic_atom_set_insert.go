package qe

import "github.com/glycerine/g3/pkg/ast"

func (s *ArithmeticAtomSet) insert(atom ast.ASTHandle, positive bool) error {
	if e := s.open(); e != nil {
		return e
	}
	seen, refs := s.positiveSeen, &s.positive
	if !positive {
		seen, refs = s.negativeSeen, &s.negative
	}
	if seen[atom] {
		return nil
	}
	k, e := s.manager.GetKind(atom)
	if e != nil || k != ast.ASTApp {
		return ast.ErrASTDomain
	}
	r, e := s.manager.NewRef(atom)
	if e != nil {
		return e
	}
	seen[atom] = true
	*refs = append(*refs, r)
	return nil
}
func (s *ArithmeticAtomSet) AddPositive(atom ast.ASTHandle) error { return s.insert(atom, true) }
func (s *ArithmeticAtomSet) AddNegative(atom ast.ASTHandle) error { return s.insert(atom, false) }
