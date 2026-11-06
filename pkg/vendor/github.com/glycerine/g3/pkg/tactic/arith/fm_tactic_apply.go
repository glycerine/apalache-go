package arith

// Source: src/tactic/arith/fm_tactic.cpp:804-846,1120-1146,1519-1618.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type fmTacticRun struct {
	diagnostics         fmConstraintDiagnostics
	manager             *ast.ASTManager
	bytes               uint64
	store               *FMConstraintStore
	parser              *FMInputParser
	eliminator          *FMEliminator
	goal                *tactic.Goal
	model               *FMModelConverter
	modelCleanup        bool
	counter, eliminated uint32
}

func newFMTacticRun(m *ast.ASTManager) (*fmTacticRun, error) {
	n := uint64(reflect.TypeOf(fmTacticRun{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	return &fmTacticRun{manager: m, bytes: n}, nil
}
func (r *fmTacticRun) close() error {
	if err := r.clear(); err != nil {
		return err
	}
	r.manager.Budget().Release(r.bytes)
	r.bytes = 0
	return nil
}
func (r *fmTacticRun) clear() error {
	if r.store != nil {
		if err := r.store.Close(); err != nil {
			return err
		}
		r.store = nil
	}
	if r.eliminator != nil {
		if err := r.eliminator.Close(); err != nil {
			return err
		}
		r.eliminator = nil
	}
	if r.parser != nil {
		if err := r.parser.Close(); err != nil {
			return err
		}
		r.parser = nil
	}
	if r.goal != nil {
		if err := r.goal.Close(); err != nil {
			return err
		}
		r.goal = nil
	}
	if r.model != nil {
		var err error
		if r.modelCleanup {
			err = r.model.Close()
		} else {
			var consumed bool
			consumed, err = r.model.DecRefConsumed()
			if consumed {
				r.modelCleanup = err != nil
			}
		}
		if err != nil {
			return err
		}
		r.model = nil
	}
	r.counter, r.eliminated, r.modelCleanup = 0, 0, false
	r.diagnostics = fmConstraintDiagnostics{}
	return nil
}

func (t *FMTactic) Apply(input *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if input == nil || result == nil || input.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = fmAssertGoalSort(input); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("fm", input)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = tactic.FailIfProofGeneration("fm", input); err != nil {
		return err
	}
	r := t.run
	if err = r.clear(); err != nil {
		return err
	}
	t.running = true
	defer func() { t.running = false }()
	r.store, err = NewFMConstraintStore(t.manager)
	if err != nil {
		return err
	}
	r.parser, err = NewFMInputParser(t.manager, r.store, t.realOnly, t.clauses)
	if err != nil {
		return err
	}
	r.diagnostics = fmConstraintDiagnostics{manager: t.manager, services: t.services, expressions: r.parser.Expressions}
	r.store.diagnostics = &r.diagnostics
	r.parser.Variables.Index.diagnostics = &r.diagnostics
	if err = r.parser.collectForbidden(input); err != nil {
		return err
	}
	r.goal, err = tactic.NewGoalConfigCopy(input)
	if err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		inDepth, e := input.Depth()
		if e != nil {
			return e
		}
		outDepth, e := r.goal.Depth()
		if e != nil {
			return e
		}
		inPrecision, e := input.Precision()
		if e != nil {
			return e
		}
		outPrecision, e := r.goal.Precision()
		if e != nil {
			return e
		}
		if inDepth != outDepth || inPrecision != outPrecision {
			return ast.ErrASTDomain
		}
	}
	if err = r.goal.IncDepth(); err != nil {
		return err
	}
	r.eliminator, err = NewFMEliminator(r.store, r.parser.Variables, t.config)
	if err != nil {
		return err
	}
	r.parser.Variables.Index.Counter = &r.counter
	r.eliminator.Counter, r.eliminator.Expressions, r.eliminator.Output = &r.counter, r.parser.Expressions, r.goal
	size, err := input.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size && !r.eliminator.Inconsistent; i++ {
		formula, e := input.Form(i)
		if e != nil {
			return e
		}
		dependency, e := input.Dependency(i)
		if e != nil {
			return e
		}
		constraint, registration, parsed, e := r.parser.Parse(formula, dependency)
		if e != nil {
			return e
		}
		if !parsed {
			if err = r.goal.AssertWithProof(formula, ast.ASTHandle{}, dependency); err != nil {
				return err
			}
			continue
		}
		switch registration {
		case FMRegistrationFalse:
			r.eliminator.Inconsistent = true
			if err = r.store.Delete(constraint); err != nil {
				return err
			}
		case FMRegistrationDeferred:
			if err = r.copyConstraint(constraint, false); err != nil {
				return err
			}
			if err = r.store.Delete(constraint); err != nil {
				return err
			}
		}
	}
	if r.eliminator.Inconsistent {
		if err = r.setFalse(t.manager); err != nil {
			return err
		}
	} else {
		if err = r.diagnostics.state(); err != nil {
			return err
		}
		if err = r.parser.Variables.Index.Subsume(); err != nil {
			return err
		}
		candidates := r.parser.Variables.Candidates()
		if err = r.diagnostics.candidates(candidates); err != nil {
			return err
		}
		models, e := input.ModelsEnabled()
		if e != nil {
			return e
		}
		if models {
			r.model, err = NewFMModelConverter(t.manager, t.services)
			if err != nil {
				return err
			}
			if err = r.model.IncRef(); err != nil {
				return err
			}
			r.eliminator.Model = r.model
		}
		for _, variable := range candidates {
			if err = t.services.Cooperation.Cooperate("fm"); err != nil {
				return err
			}
			if !t.manager.ResourceLimit().Inc() {
				return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
			}
			memory, e := t.services.Memory.AllocationSize()
			if e != nil {
				return e
			}
			if memory > t.maxMemory {
				return tactic.NewTacticException("max. memory exceeded")
			}
			if r.counter > t.limit {
				break
			}
			r.counter++
			eliminated, e := r.eliminator.Try(variable)
			if e != nil {
				return e
			}
			if eliminated {
				r.eliminated++
			}
			if r.eliminator.Inconsistent {
				if err = r.setFalse(t.manager); err != nil {
					return err
				}
				break
			}
		}
		if err = tactic.ReportTacticProgress(":fm-eliminated", r.eliminated); err != nil {
			return err
		}
		if err = tactic.ReportTacticProgress(":fm-cost", r.counter); err != nil {
			return err
		}
		if !r.eliminator.Inconsistent {
			for _, upper := range []bool{true, false} {
				for x := range r.parser.Variables.Integer {
					cs := r.parser.Variables.Index.Upper[uint32(x)]
					if !upper {
						cs = r.parser.Variables.Index.Lower[uint32(x)]
					}
					for _, c := range cs {
						if !c.Dead {
							c.Dead = true
							if err = r.copyConstraint(c, true); err != nil {
								return err
							}
						}
					}
				}
			}
			if r.model != nil {
				if err = result.SetModelConverter(r.model); err != nil {
					return err
				}
			}
		}
	}
	if err = r.store.Close(); err != nil {
		return err
	}
	r.store = nil
	published := r.goal
	if err = result.TakeGoal(r.goal); err != nil {
		return err
	}
	r.goal = nil
	if err = r.diagnostics.goal(published); err != nil {
		return err
	}
	return fmAssertGoalSort(published)
}
func (r *fmTacticRun) copyConstraint(c *FMConstraint, remaining bool) error {
	formula, err := r.parser.Expressions.ConstraintExpression(c)
	if err != nil {
		return err
	}
	if remaining {
		if err = r.diagnostics.copied(formula, c.Dependency); err != nil {
			return err
		}
	}
	return r.goal.AssertWithProof(formula, ast.ASTHandle{}, c.Dependency)
}
func (r *fmTacticRun) setFalse(m *ast.ASTManager) error {
	if err := r.goal.Reset(); err != nil {
		return err
	}
	value, err := m.MkFalse()
	if err != nil {
		return err
	}
	return r.goal.AssertWithProof(value, ast.ASTHandle{}, r.eliminator.InconsistentDependency)
}

func fmAssertGoalSort(goal *tactic.Goal) error {
	if !ast.ASTSourceDebug {
		return nil
	}
	good, err := goal.IsWellSorted()
	if err != nil {
		return err
	}
	if !good {
		return ast.ErrASTDomain
	}
	return nil
}
