package rewriter

// Source: enum2bv_rewriter.cpp:162-195, concrete bounds/new_body_ref owners.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

type enum2BVQuantifierScope struct {
	manager *ast.ASTManager
	bounds  []ast.ASTRef
	values  []ast.ASTHandle
	body    ast.ASTRef
}

func (s *enum2BVQuantifierScope) push(value ast.ASTHandle) error {
	ref, err := s.manager.NewRef(value)
	if err != nil {
		return err
	}
	s.bounds = append(s.bounds, ref)
	s.values = append(s.values, value)
	return nil
}
func (s *enum2BVQuantifierScope) close() error {
	// Source tmp stays null. The later new_body_ref dies before bounds.
	if s.body != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.body); err != nil {
			return err
		}
	}
	for i := range s.bounds {
		if s.bounds[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.bounds[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *Enum2BVRewriter) finishQuantifier(s *enum2BVQuantifierScope, result *error) {
	err := s.close()
	if err != nil {
		r.pending = append(r.pending, enum2BVPending{quantifier: s})
		r.closing = true
	}
	*result = errors.Join(*result, err)
}
