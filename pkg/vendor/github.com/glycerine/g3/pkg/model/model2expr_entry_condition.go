package model

// Source: src/model/model2expr.cpp:65-77; src/util/ref_vector.h:43-56,96-100.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// modelEntryCondition owns the concrete expr_ref_vector of equality terms.
// A stopped decrement leaves its unconsumed suffix available for Close retry.
type modelEntryCondition struct {
	manager *ast.ASTManager
	roots   []ast.ASTHandle
	active  bool
}

func newModelEntryCondition(m *ast.ASTManager, arity uint32, entry *FunctionEntry, out ast.ASTRef) (*modelEntryCondition, error) {
	f := &modelEntryCondition{manager: m, active: true}
	if e := entry.open(); e != nil {
		return f, e
	}
	for i := uint32(0); i < arity; i++ {
		arg, e := entry.Argument(i)
		if e != nil {
			return f, e
		}
		kind, e := m.GetKind(arg)
		if e != nil {
			return f, e
		}
		if kind == ast.ASTVar {
			index, e := m.GetVarIndex(arg)
			if e != nil {
				return f, e
			}
			if index == i {
				continue
			}
		}
		sort, e := m.GetSort(arg)
		if e != nil {
			return f, e
		}
		v, e := m.MkVar(i, sort)
		if e != nil {
			return f, e
		}
		eq, e := m.MkEq(v, arg)
		if e != nil {
			return f, e
		}
		if e = appendArrayRoot(m, &f.roots, eq); e != nil {
			return f, e
		}
	}
	return f, rewriter.DefaultAnd(m, f.roots, out)
}

func (f *modelEntryCondition) Close() error {
	if f == nil || !f.active {
		return nil
	}
	if e := releaseArrayRoots(f.manager, &f.roots); e != nil {
		return e
	}
	f.active = false
	return nil
}
