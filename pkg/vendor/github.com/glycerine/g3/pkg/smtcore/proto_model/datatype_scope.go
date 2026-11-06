package protomodel

// Source: expr_ref_vector args and expr_ref new_value in datatype_factory.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type datatypeFactoryPending struct {
	scope    *datatypeFactoryScope
	contains *ast.ContainsApp
	trace    *datatypeFactoryTraceWriter
}

type datatypeFactoryScope struct {
	manager   *ast.ASTManager
	values    []ast.ASTHandle
	args      []ast.ASTRef
	candidate ast.ASTRef
}

func (s *datatypeFactoryScope) push(value ast.ASTHandle) error {
	ref, err := s.manager.NewRef(value)
	if err != nil {
		return err
	}
	s.args = append(s.args, ref)
	s.values = append(s.values, value)
	return nil
}
func (s *datatypeFactoryScope) ownCandidate(value ast.ASTHandle) error {
	var err error
	s.candidate, err = s.manager.NewRef(value)
	return err
}
func (s *datatypeFactoryScope) close() error {
	// The later expr_ref dies first; project ref_vector then walks forward.
	if s.candidate != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.candidate); err != nil {
			return err
		}
	}
	for i := range s.args {
		if s.args[i] == (ast.ASTRef{}) {
			continue
		}
		if err := s.manager.DelRef(&s.args[i]); err != nil {
			return err
		}
	}
	s.args = nil
	s.values = nil
	return nil
}
func (f *DatatypeFactory) finishScope(s *datatypeFactoryScope, result *error) {
	err := s.close()
	if err != nil {
		f.pending = append(f.pending, datatypeFactoryPending{scope: s})
	}
	*result = errors.Join(*result, err)
}

func (f *DatatypeFactory) finishContains(contains *ast.ContainsApp, result *error) {
	err := contains.Close()
	if err != nil {
		f.pending = append(f.pending, datatypeFactoryPending{contains: contains})
	}
	*result = errors.Join(*result, err)
}
