//go:build g3debug

package ast

import "github.com/glycerine/g3/pkg/util/diagnostic"

const ASTSourceDebug = true

func (m *ASTManager) registrationBefore(r astNodeRef) (bool, error) {
	_, contains, e := m.tableFind(r)
	if e != nil {
		return false, e
	}
	if !contains {
		debug := diagnostic.BorrowProcessDebugContext()
		enabled, e := debug.ConditionalAssertionEnabled("nondet_bug")
		if e != nil {
			return false, e
		}
		if !enabled {
			return contains, nil
		}
		count := uint32(0)
		for _, curr := range m.table.entries.All() {
			equal, e := m.compareNodes(curr, r)
			if e != nil {
				return false, e
			}
			if equal {
				if e := m.traceNondeterminismCollision(curr, r); e != nil {
					return false, e
				}
				return false, diagnostic.DebugAssertionFailure(debug, "src/ast/ast.cpp", 1671, "contains || slow_not_contains(n)")
			}
			if m.nodeAt(curr.slot).kind == ASTApp && m.nodeAt(r.slot).kind == ASTApp {
				a, b := m.appAt(curr.slot), m.appAt(r.slot)
				if a.decl == b.decl && a.count == 0 && b.count == 0 {
					return false, ErrASTDomain
				}
			}
			count++
		}
		if count != m.table.size {
			return false, ErrASTDomain
		}
	}
	return contains, nil
}
func (m *ASTManager) registrationAfter(candidate, result astNodeRef, contains, unique bool) error {
	if m.nodeAt(candidate.slot).hash != m.nodeAt(result.slot).hash {
		return ErrASTDomain
	}
	if unique == contains {
		return ErrASTDomain
	}
	_, found, e := m.tableFind(candidate)
	if e != nil {
		return e
	}
	if !found {
		return ErrASTDomain
	}
	return nil
}
func (m *ASTManager) deletionBefore(r astNodeRef) error {
	_, yes, e := m.tableFind(r)
	if e != nil {
		return e
	}
	if !yes {
		return ErrASTDomain
	}
	return nil
}
func (m *ASTManager) deletionAfter(r astNodeRef) error {
	_, yes, e := m.tableFind(r)
	if e != nil {
		return e
	}
	if yes {
		return ErrASTDomain
	}
	return nil
}
