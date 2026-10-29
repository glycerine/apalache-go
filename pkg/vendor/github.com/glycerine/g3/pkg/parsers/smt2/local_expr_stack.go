package smt2

// Source: src/parsers/smt2/smt2parser.cpp:236-240,1700-1718,2818-2832,2992-3003.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var errLocalExprStack = errors.New("smt2 parser: expression stack position is invalid")

// localExprStack is the parser's concrete expr_ref_vector owner. It takes an
// existing local expression reference without acquiring a second reference.
type localExprStack struct{ refs []ast.ASTRef }

func (s *localExprStack) pushOwned(ref ast.ASTRef) { s.refs = append(s.refs, ref) }

func (s *localExprStack) top(m *ast.ASTManager) (ast.ASTHandle, error) {
	if s == nil || len(s.refs) == 0 {
		return ast.ASTHandle{}, errLocalExprStack
	}
	return m.RefGet(s.refs[len(s.refs)-1])
}

// shrink releases removed expression slots in source-forward order. A pending
// release retains its slot so a later cleanup can retry it.
func (s *localExprStack) shrink(m *ast.ASTManager, size int) error {
	if s == nil || size < 0 {
		return errLocalExprStack
	}
	if err := sourceAssertLocalRefVector(size <= len(s.refs), 91, "sz <= m_nodes.size()"); err != nil {
		return err
	}
	if size > len(s.refs) {
		return errLocalExprStack
	}
	return s.releaseSuffix(m, size)
}

func (s *localExprStack) releaseSuffix(m *ast.ASTManager, size int) error {
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

func (s *localExprStack) reset(m *ast.ASTManager) error {
	if s == nil {
		return errLocalExprStack
	}
	return s.releaseSuffix(m, 0)
}
