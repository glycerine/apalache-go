package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type DegreeShiftTactic struct {
	self       *DegreeShiftTactic
	manager    *ast.ASTManager
	services   model.ModelEvaluatorConfig
	trace      *diagnostic.TraceTags
	ownerBytes uint64
	refs       uint32
	closed     bool
}

var _ tactic.Tactic = (*DegreeShiftTactic)(nil)

func NewDegreeShiftTactic(m *ast.ASTManager, services model.ModelEvaluatorConfig) (*DegreeShiftTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(DegreeShiftTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &DegreeShiftTactic{manager: m, services: services, ownerBytes: n}
	t.self = t
	return t, nil
}
func (t *DegreeShiftTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *DegreeShiftTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *DegreeShiftTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *DegreeShiftTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *DegreeShiftTactic) Cleanup() error                                { return t.open() }
func (t *DegreeShiftTactic) Reset() error                                  { return t.open() }
func (t *DegreeShiftTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DegreeShiftTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DegreeShiftTactic) CollectStatistics() map[string]float64         { return nil }
func (t *DegreeShiftTactic) ResetStatistics() error                        { return t.open() }
func (t *DegreeShiftTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DegreeShiftTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *DegreeShiftTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *DegreeShiftTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewDegreeShiftTactic(m, t.services)
}

func (t *DegreeShiftTactic) checkpoint() error {
	canceled, err := t.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
	}
	if t.services.Cooperation != nil {
		return t.services.Cooperation.Cooperate("degree_shift")
	}
	return nil
}

func (t *DegreeShiftTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != t.manager || result == nil {
		return ast.ErrASTHandle
	}
	m := t.manager
	if err = result.Prepare(m); err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	roots := make([]ast.ASTHandle, size)
	for i := range roots {
		roots[i], err = goal.Form(uint32(i))
		if err != nil {
			return err
		}
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("degree_shift", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	plan, err := degreeShiftPrepareWithCheckpoint(m, roots, proofs, t.checkpoint)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, plan.Close()) }()
	if err = t.traceCandidates(plan); err != nil {
		return err
	}
	if len(plan.entries) != 0 {
		walk, e := newDegreeShiftTraversal(plan, proofs)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, walk.Close()) }()
		for i, root := range roots {
			if e := t.checkpoint(); e != nil {
				return e
			}
			out, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return e
			}
			pr, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return errors.Join(e, m.DelRef(&out))
			}
			if proofs {
				e = walk.RewriteWithProof(root, out, pr)
			} else {
				e = walk.Rewrite(root, out)
			}
			if e == nil {
				var value, proof ast.ASTHandle
				value, e = m.RefGet(out)
				if e == nil && proofs {
					var old, rewrite ast.ASTHandle
					old, e = goal.Proof(uint32(i))
					if e == nil {
						rewrite, e = m.RefGet(pr)
					}
					if e == nil {
						proof, e = m.MkProofModusPonens(old, rewrite)
					}
				}
				if e == nil {
					var dep ast.ASTDependency
					dep, e = goal.Dependency(uint32(i))
					if e == nil {
						e = goal.Update(uint32(i), value, proof, dep)
					}
				}
			}
			e = errors.Join(e, m.DelRef(&pr), m.DelRef(&out))
			if e != nil {
				return e
			}
		}
		if err = degreeShiftAssertBounds(plan, goal, proofs); err != nil {
			return err
		}
		if models {
			if err = degreeShiftPublishModel(plan, t.services, result); err != nil {
				return err
			}
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	return t.traceResult(goal, result)
}
