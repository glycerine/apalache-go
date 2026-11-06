package smt2

// Source: src/parsers/smt2/smt2parser.cpp:230-234,1723-1729.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var errLocalSortStack = errors.New("smt2 parser: qualified sort stack is empty")

// localSortStack is the parser's concrete sort_ref_vector owner. Native slice
// slots retain AST references until the parser consumes or clears them.
type localSortStack struct{ refs []ast.ASTRef }

func (s *localSortStack) push(m *ast.ASTManager, sort ast.ASTHandle) error {
	ref, err := m.NewRef(sort)
	if err != nil {
		return err
	}
	s.refs = append(s.refs, ref)
	return nil
}

func (s *localSortStack) top(m *ast.ASTManager) (ast.ASTHandle, error) {
	if s == nil || len(s.refs) == 0 {
		return ast.ASTHandle{}, errLocalSortStack
	}
	return m.RefGet(s.refs[len(s.refs)-1])
}

func (s *localSortStack) pop(m *ast.ASTManager) error {
	if s == nil {
		return errLocalSortStack
	}
	if err := sourceAssertLocalRefVector(len(s.refs) != 0, 103, "!m_nodes.empty()"); err != nil {
		return err
	}
	if len(s.refs) == 0 {
		return errLocalSortStack
	}
	last := len(s.refs) - 1
	err := m.DelRef(&s.refs[last])
	if s.refs[last] == (ast.ASTRef{}) {
		s.refs = s.refs[:last]
	}
	return err
}

// shrink mirrors ref_vector_core::shrink: release discarded slots in their
// original order. If a release is pending, keep its slot for a later reset.
func (s *localSortStack) shrink(m *ast.ASTManager, size int) error {
	if s == nil || size < 0 {
		return errLocalSortStack
	}
	if err := sourceAssertLocalRefVector(size <= len(s.refs), 91, "sz <= m_nodes.size()"); err != nil {
		return err
	}
	if size > len(s.refs) {
		return errLocalSortStack
	}
	return s.releaseSuffix(m, size)
}

func (s *localSortStack) releaseSuffix(m *ast.ASTManager, size int) error {
	var failure error
	for i := size; i < len(s.refs); i++ {
		if s.refs[i] == (ast.ASTRef{}) {
			continue
		}
		failure = errors.Join(failure, m.DelRef(&s.refs[i]))
	}
	if failure == nil {
		s.refs = s.refs[:size]
	}
	return failure
}

func (s *localSortStack) reset(m *ast.ASTManager) error {
	if s == nil {
		return errLocalSortStack
	}
	return s.releaseSuffix(m, 0)
}

func sourceAssertLocalRefVector(ok bool, line int32, expression string) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/ref_vector.h", line, expression)
}
