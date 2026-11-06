package qe

import "github.com/glycerine/g3/pkg/ast"

func (s *ArithmeticAtomSet) handles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	out := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		h, e := s.manager.RefGet(refs[i])
		if e != nil {
			return nil, e
		}
		out[i] = h
	}
	return out, nil
}
func (s *ArithmeticAtomSet) Positive() ([]ast.ASTHandle, error) { return s.handles(s.positive) }
func (s *ArithmeticAtomSet) Negative() ([]ast.ASTHandle, error) { return s.handles(s.negative) }
