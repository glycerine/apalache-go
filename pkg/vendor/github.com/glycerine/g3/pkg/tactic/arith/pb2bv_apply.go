package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp imp::operator() and is_pb_probe.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (t *PB2BVTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	defer func() {
		var unsupported pbNonPB
		if errors.As(err, &unsupported) {
			err = t.unsupported(unsupported.term)
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.emit("pb2bv", "operator()", 893, goal.Display); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		if yes, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !yes {
			return ast.ErrASTDomain
		}
	}
	if err = tactic.FailIfProofGeneration("pb2bv", goal); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("pb2bv", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.bounds.Reset(); err != nil {
		return err
	}
	if err = t.traversal.Reset(); err != nil {
		return err
	}
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return err
	}
	if inconsistent {
		return result.AddGoal(goal)
	}
	if err = t.bounds.CollectGoal(goal); err != nil {
		return err
	}
	if err = t.emit("pb2bv", "operator()", 909, t.bounds.Display); err != nil {
		return err
	}
	if _, err = t.check(goal); err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	var forms []ast.ASTRef
	var deps []ast.ASTDependency
	defer func() {
		for i := len(forms) - 1; i >= 0; i-- {
			err = errors.Join(err, t.manager.DelRef(&forms[i]))
		}
		for i := len(deps) - 1; i >= 0; i-- {
			err = errors.Join(err, t.manager.DependencyDec(deps[i]))
		}
	}()
	current, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&current)) }()
	for i := uint32(0); i < size; i++ {
		term, e := goal.Form(i)
		if e != nil {
			return e
		}
		atom, positive := term, true
		for {
			f, op, args, app, e := t.bounds.app(atom)
			if e != nil {
				return e
			}
			if !app || f != t.manager.BasicFamilyID() || op != ast.ASTOpNot || len(args) != 1 {
				break
			}
			atom = args[0]
			positive = !positive
		}
		constraint, e := t.constraintCore(atom)
		if e != nil {
			return e
		}
		if constraint {
			if err = t.convert(atom, current, positive, true); err != nil {
				return err
			}
			value, e := t.manager.RefGet(current)
			if e != nil {
				return e
			}
			if err = t.emit("pb2bv_convert", "operator()", 932, func(out io.Writer) error {
				flag := 0
				if positive {
					flag = 1
				}
				if _, err := fmt.Fprintf(out, "pos: %d\n", flag); err != nil {
					return err
				}
				if err := t.bounds.writeExpr(out, atom); err != nil {
					return err
				}
				if _, err := io.WriteString(out, "\n--->\n"); err != nil {
					return err
				}
				if err := t.bounds.writeExpr(out, value); err != nil {
					return err
				}
				_, err := io.WriteString(out, "\n")
				return err
			}); err != nil {
				return err
			}
		} else {
			if err = t.traversal.Rewrite(term, current); err != nil {
				return err
			}
		}
		if cores {
			old, e := goal.Dependency(i)
			if e != nil {
				return e
			}
			dep, e := t.manager.DependencyJoin(t.used, old)
			if e != nil {
				return e
			}
			if err = t.manager.DependencyInc(dep); err != nil {
				return err
			}
			deps = append(deps, dep)
			if err = t.resetUsed(); err != nil {
				return err
			}
		}
		value, e := t.manager.RefGet(current)
		if e != nil {
			return e
		}
		ref, e := t.manager.NewRef(value)
		if e != nil {
			return e
		}
		forms = append(forms, ref)
	}
	for i := uint32(0); i < size; i++ {
		form, e := t.manager.RefGet(forms[i])
		if e != nil {
			return e
		}
		var dep ast.ASTDependency
		if cores {
			dep = deps[i]
		} else {
			dep, e = goal.Dependency(i)
			if e != nil {
				return e
			}
		}
		if err = goal.Update(i, form, ast.ASTHandle{}, dep); err != nil {
			return err
		}
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	if models {
		if err = t.publishModel(result); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if err = t.emit("pb2bv", "operator()", 967, goal.Display); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		if yes, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !yes {
			return ast.ErrASTDomain
		}
	}
	return nil
}
func (t *PB2BVTactic) publishModel(result *tactic.TacticResult) (err error) {
	m := t.manager
	filter, err := tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = filter.IncRef(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	if err = filter.BindTrace(t.trace, t.globals); err != nil {
		return err
	}
	bindings := make([]PB2BVBinding, 0, len(t.entries))
	for _, entry := range t.entries {
		decl, e := m.GetDecl(entry.positive)
		if e != nil {
			return e
		}
		if err = filter.Insert(decl); err != nil {
			return err
		}
		bindings = append(bindings, PB2BVBinding{entry.declaration, decl})
	}
	for _, variable := range t.temporaryInts {
		decl, e := m.GetDecl(variable)
		if e != nil {
			return e
		}
		if err = filter.Insert(decl); err != nil {
			return err
		}
	}
	converter, err := NewPB2BVModelConverter(m, t.context, t.globals, bindings, t.bounds)
	if err != nil {
		return err
	}
	converter.trace = t.trace
	if err = converter.IncRef(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, converter.DecRef()) }()
	combined, err := tactic.ConcatModels(m.Budget(), filter, converter)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(combined); err != nil {
		return errors.Join(err, combined.Close())
	}
	return nil
}

// IsPseudoBooleanGoal is the concrete body of the retained is-pb probe.
func IsPseudoBooleanGoal(goal *tactic.Goal, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (yes bool, err error) {
	if goal == nil {
		return false, ast.ErrASTHandle
	}
	bounds, err := NewBoundManager(goal.Manager(), c, g)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, bounds.Close()) }()
	if err = bounds.CollectGoal(goal); err != nil {
		return false, err
	}
	checker := PB2BVTactic{manager: goal.Manager(), context: c, globals: g, bounds: bounds}
	yes, err = checker.check(goal)
	var unsupported pbNonPB
	if errors.As(err, &unsupported) {
		return false, nil
	}
	return yes, err
}
