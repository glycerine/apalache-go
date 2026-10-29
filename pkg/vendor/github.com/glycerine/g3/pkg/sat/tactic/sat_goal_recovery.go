package tactic

// Source: src/sat/tactic/atom2bool_var.cpp:25-36;
// src/sat/tactic/goal2sat.cpp:685-779; src/util/ref_vector.h:67-71,127-131;
// src/util/vector.h:422-434; src/util/buffer.h:25-48.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
)

// Concrete expr_ref_vector used by the inverse map and SAT recovery. Each
// populated slot owns a reference, including both polarities of each atom.
type satGoalLiteralExpressions struct{ values []ast.ASTHandle }

func (v *satGoalLiteralExpressions) resize(m *ast.ASTManager, n uint32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	newLen := int(n)
	if newLen < len(v.values) {
		for i := newLen; i < len(v.values); i++ {
			if !v.values[i].IsNull() {
				if e := m.DecRef(v.values[i]); e != nil {
					return e
				}
				v.values[i] = ast.ASTHandle{}
			}
		}
	}
	oldLen := len(v.values)
	if newLen > cap(v.values) {
		data := make([]ast.ASTHandle, newLen)
		copy(data, v.values)
		v.values = data
	} else {
		v.values = v.values[:newLen]
		for i := oldLen; i < newLen; i++ {
			v.values[i] = ast.ASTHandle{}
		}
	}
	return nil
}
func (v *satGoalLiteralExpressions) set(m *ast.ASTManager, i uint32, h ast.ASTHandle) error {
	if uint64(i) >= uint64(len(v.values)) {
		return arena.ErrBounds
	}
	if !h.IsNull() {
		if e := m.IncRef(h); e != nil {
			return e
		}
	}
	old := v.values[i]
	if !old.IsNull() {
		if e := m.DecRef(old); e != nil {
			if !h.IsNull() {
				return errors.Join(e, m.DecRef(h))
			}
			return e
		}
	}
	v.values[i] = h
	return nil
}
func (v *satGoalLiteralExpressions) close(m *ast.ASTManager) error {
	if e := v.resize(m, 0); e != nil {
		return e
	}
	v.values = nil
	return nil
}
func (tr *GoalTranslator) inverseAtoms(out *satGoalLiteralExpressions) error {
	if e := tr.open(); e != nil {
		return e
	}
	entries, err := tr.atomEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		h, value := entry.Expr, entry.Variable
		l := sat.MakeLiteral(sat.BoolVar(value), false)
		if e := out.set(tr.manager, l.Index(), h); e != nil {
			return e
		}
		neg, e := tr.manager.MkNot(h)
		if e != nil {
			return e
		}
		if e = out.set(tr.manager, l.Negated().Index(), neg); e != nil {
			return e
		}
	}
	return nil
}

// Formula-only entry point used by callers that do not request a model.
func recoverSATGoalFormulas(tr *GoalTranslator, g *tactic.Goal, learned bool, maxMemory uint64) (err error) {
	return recoverSATGoal(tr, g, learned, maxMemory, model.ModelEvaluatorConfig{}, nil)
}

// RecoverSATGoal implements sat2goal, including the optional model converter.
// The caller owns a returned converter even when later clause recovery fails.
func RecoverSATGoal(tr *GoalTranslator, g *tactic.Goal, learned bool, maxMemory uint64, services model.ModelEvaluatorConfig) (mc tactic.ModelConverter, err error) {
	err = recoverSATGoal(tr, g, learned, maxMemory, services, &mc)
	return mc, err
}

func recoverSATGoal(tr *GoalTranslator, g *tactic.Goal, learned bool, maxMemory uint64, services model.ModelEvaluatorConfig, out *tactic.ModelConverter) (err error) {
	if err = tr.open(); err != nil {
		return err
	}
	if err = g.CheckOwner(); err != nil {
		return err
	}
	if g.Manager() != tr.manager {
		return ast.ErrASTHandle
	}
	s, m := tr.solver, tr.manager
	inconsistent, e := s.SourceInconsistent()
	if e != nil {
		return e
	}
	if inconsistent {
		f, e := m.MkFalse()
		if e != nil {
			return e
		}
		return g.Assert(f)
	}
	checkpoint := func() error {
		canceled, e := m.Canceled()
		if e != nil {
			return e
		}
		if canceled {
			return tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		bytes, e := s.RuntimeOwner().AllocationSize()
		if e != nil {
			return e
		}
		if bytes > maxMemory {
			return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
		}
		return nil
	}
	var expressions satGoalLiteralExpressions
	defer func() { err = errors.Join(err, expressions.close(m)) }()
	var converter *SATModelConverter
	published := false
	models, e := g.ModelsEnabled()
	if e != nil {
		return e
	}
	if out != nil && models {
		converter, err = NewSATModelConverter(m, s, services)
		if err != nil {
			if converter != nil {
				*out = converter
			}
			return err
		}
		defer func() {
			if !published {
				if cleanup := converter.Close(); cleanup != nil {
					*out = converter
					err = errors.Join(err, cleanup)
				}
			}
		}()
	}
	n, e := s.SourceNumVars()
	if e != nil {
		return e
	}
	if n > ^uint32(0)/2 {
		return arena.ErrBounds
	}
	if e = expressions.resize(m, n*2); e != nil {
		return e
	}
	if e = tr.inverseAtoms(&expressions); e != nil {
		return e
	}
	for v := uint32(0); v < n; v++ {
		if e = checkpoint(); e != nil {
			return e
		}
		if expressions.values[2*v].IsNull() {
			if !expressions.values[2*v+1].IsNull() {
				return sat.ErrSATAssertion
			}
			aux, e := m.MkFreshConstNullable(nil, tr.boolSort)
			if e != nil {
				return e
			}
			if converter != nil {
				if e = converter.Insert(aux, true); e != nil {
					return e
				}
			}
			if e = expressions.set(m, 2*v, aux); e != nil {
				return e
			}
			neg, e := m.MkNot(aux)
			if e != nil {
				return e
			}
			if e = expressions.set(m, 2*v+1, neg); e != nil {
				return e
			}
		} else {
			if converter != nil {
				if e = converter.Insert(expressions.values[2*v], false); e != nil {
					return e
				}
			}
			if expressions.values[2*v+1].IsNull() {
				return sat.ErrSATAssertion
			}
		}
	}
	if out != nil && converter != nil {
		*out = converter
		published = true
	}
	for v := uint32(0); v < n; v++ {
		if e = checkpoint(); e != nil {
			return e
		}
		value, e := s.SourceValueVar(sat.BoolVar(v))
		if e != nil {
			return e
		}
		switch value {
		case lbool.True:
			e = g.Assert(expressions.values[2*v])
		case lbool.False:
			e = g.Assert(expressions.values[2*v+1])
		}
		if e != nil {
			return e
		}
	}
	bins, e := s.CollectBinClauses(nil, learned)
	if e != nil {
		return e
	}
	for _, pair := range bins {
		if e = checkpoint(); e != nil {
			return e
		}
		f, e := m.MkOr(expressions.values[pair.FirstIndex()], expressions.values[pair.SecondIndex()])
		if e != nil {
			return e
		}
		if e = g.Assert(f); e != nil {
			return e
		}
	}
	assertClauses := func(clauses []sat.StoredClauseRef) (err error) {
		// The literal handles are borrowed for each MkOrArgs call. The goal
		// owns the resulting formula after Assert publishes it.
		var lits []ast.ASTHandle
		for _, clause := range clauses {
			if e := checkpoint(); e != nil {
				return e
			}
			lits = lits[:0]
			size, e := s.ClauseAllocator().Size(clause)
			if e != nil {
				return e
			}
			for i := uint32(0); i < size; i++ {
				literal, e := s.ClauseAllocator().Literal(clause, i)
				if e != nil {
					return e
				}
				lits = append(lits, expressions.values[literal.Index()])
			}
			f, e := m.MkOrArgs(lits)
			if e != nil {
				return e
			}
			if e = g.Assert(f); e != nil {
				return e
			}
		}
		return nil
	}
	if e = assertClauses(s.ClauseReferences(false)); e != nil {
		return e
	}
	if learned {
		return assertClauses(s.ClauseReferences(true))
	}
	return nil
}
