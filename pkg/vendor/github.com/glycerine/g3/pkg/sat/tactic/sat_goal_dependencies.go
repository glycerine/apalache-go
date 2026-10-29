package tactic

// Source: src/sat/tactic/goal2sat.cpp:425-485;
// src/sat/tactic/atom2bool_var.cpp:88-111; src/sat/tactic/sat_tactic.cpp:133-138.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
)

func (tr *GoalTranslator) retainDependency(h ast.ASTHandle) error {
	if e := tr.manager.IncRef(h); e != nil {
		return e
	}
	tr.dependencyTrail = append(tr.dependencyTrail, h)
	return nil
}
func (tr *GoalTranslator) processOwning(h ast.ASTHandle) (err error) {
	if err = tr.manager.IncRef(h); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, tr.manager.DecRef(h)) }()
	return tr.process(h)
}
func (tr *GoalTranslator) insertGoalDependency(original, dep ast.ASTHandle, sign bool) (err error) {
	m := tr.manager
	simple, e := tr.isUninterpretedConstant(dep)
	if e != nil {
		return e
	}
	fresh := dep
	if !simple {
		fresh, e = m.MkFreshConst("dep", tr.boolSort)
		if e != nil {
			return e
		}
	}
	if e = m.IncRef(fresh); e != nil {
		return e
	}
	defer func() { err = errors.Join(err, m.DecRef(fresh)) }()
	if !simple {
		if e = tr.retainDependency(fresh); e != nil {
			return e
		}
		tr.interfaces[fresh] = true
		iff, e := m.MkIff(fresh, dep)
		if e != nil {
			return e
		}
		if e = tr.processOwning(iff); e != nil {
			return e
		}
	}
	if e = tr.convertAtom(fresh, false, false); e != nil {
		return e
	}
	l := tr.results[len(tr.results)-1]
	if sign {
		l = l.Negated()
	}
	tr.dependencies.Set(original, l.Index())
	tr.results = tr.results[:len(tr.results)-1]
	return nil
}

// Dependencies are borrowed map keys, as in source dep2asm. The caller keeps
// its labels alive while consuming the assumption/core mapping.
func (tr *GoalTranslator) AppendDependencyAssumptions(out []sat.Literal) ([]sat.Literal, error) {
	if e := tr.open(); e != nil {
		return out, e
	}
	for _, value := range tr.dependencies.All() {
		out = append(out, sat.Literal(value))
	}
	return out, nil
}
func (tr *GoalTranslator) CompileGoal(g *tactic.Goal) (err error) {
	if err = tr.open(); err != nil {
		return err
	}
	if err = g.CheckOwner(); err != nil {
		return err
	}
	if g.Manager() != tr.manager {
		return ast.ErrASTHandle
	}
	defer func() {
		if err != nil {
			tr.failed = true
		}
	}()
	m := tr.manager
	var deps, forms, allDeps []ast.ASTHandle
	size, e := g.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		d, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if !d.IsNull() {
			deps, e = m.DependencyLinearize(d, deps[:0])
			if e != nil {
				return e
			}
			allDeps = append(allDeps, deps...)
		}
		f, e := g.Form(i)
		if e != nil {
			return e
		}
		forms = append(forms, f)
	}
	clear(tr.interfaces)
	if e = tr.collectInterfaceWithDependencies(forms, allDeps); e != nil {
		return e
	}
	var fmls satGoalLiteralExpressions
	defer func() { err = errors.Join(err, fmls.close(m)) }()
	for i := uint32(0); i < size; i++ {
		f, e := g.Form(i)
		if e != nil {
			return e
		}
		d, e := g.Dependency(i)
		if e != nil {
			return e
		}
		skip := false
		if !d.IsNull() {
			deps, e = m.DependencyLinearize(d, deps[:0])
			if e != nil {
				return e
			}
			if e = fmls.resize(m, 0); e != nil {
				return e
			}
			if e = fmls.resize(m, 1); e != nil {
				return e
			}
			if e = fmls.set(m, 0, f); e != nil {
				return e
			}
			for _, original := range deps {
				sign, inner, e := m.IsNot(original)
				if e != nil {
					return e
				}
				if !sign {
					inner = original
				}
				if e = tr.insertGoalDependency(original, inner, sign); e != nil {
					return e
				}
				if original == f {
					skip = true
					break
				}
				guard := inner
				if !sign {
					guard, e = m.MkNot(original)
					if e != nil {
						return e
					}
				}
				count := uint32(len(fmls.values))
				if e = fmls.resize(m, count+1); e != nil {
					return e
				}
				if e = fmls.set(m, count, guard); e != nil {
					return e
				}
			}
			if !skip {
				f, e = m.MkOrArgs(fmls.values)
				if e != nil {
					return e
				}
			}
		}
		if !skip {
			if e = tr.traceGoalCompiledFormula(f); e != nil {
				return e
			}
			if e = tr.processOwning(f); e != nil {
				return e
			}
		}
	}
	return nil
}
