package arith

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:39-62,529-549.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// PropagateIneqsTactic supplies the source virtual tactic boundary around the
// concrete bound-propagation implementation. It owns its private unsynchronized
// numeral manager, while borrowing the AST and parameter services.
type PropagateIneqsTactic struct {
	self       *PropagateIneqsTactic
	core       *PropagateIneqTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	numbers    *numeral.RationalManager
	pending    []*tactic.Goal
	ownerBytes uint64
	refs       uint32
	closed     bool
}

var _ tactic.Tactic = (*PropagateIneqsTactic)(nil)

func NewPropagateIneqsTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (*PropagateIneqsTactic, error) {
	return newPropagateIneqsAdapter(m, c, p, 0)
}

func newPropagateIneqsAdapter(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, sourceLine int) (*PropagateIneqsTactic, error) {
	if m == nil || c == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(PropagateIneqsTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &PropagateIneqsTactic{manager: m, context: c, ownerBytes: bytes}
	t.self = t
	var err error
	t.numbers, err = numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: m.Budget()},
		Words: arena.VectorConfig{Descriptors: arena.Config{Budget: m.Budget()}},
	}})
	if err == nil {
		t.core, err = newPropagateIneqTactic(m, t.numbers, c, p, sourceLine)
	}
	if err != nil {
		// A stopped numeral constructor can return its charged child owner
		// as the error, rather than publishing a rational manager.
		var failed memory.ConstructionFailure
		if errors.As(err, &failed) {
			err = errors.Join(err, failed.Cleanup())
		}
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakePropagateIneqsTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (tactic.Tactic, error) {
	child, err := newPropagateIneqsAdapter(m, c, p, 61)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func (t *PropagateIneqsTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.core.open()
}

func (t *PropagateIneqsTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *PropagateIneqsTactic) DecRef() error {
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

func (t *PropagateIneqsTactic) drainGoals() error {
	for len(t.pending) != 0 {
		if err := t.pending[0].Close(); err != nil {
			return err
		}
		t.pending = t.pending[1:]
	}
	return nil
}

func (t *PropagateIneqsTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.drainGoals(); err != nil {
		return err
	}
	if t.core != nil {
		if err := t.core.Close(); err != nil {
			return err
		}
		t.core = nil
	}
	if t.numbers != nil {
		if err := t.numbers.CloseChecked(); err != nil {
			return err
		}
		t.numbers = nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *PropagateIneqsTactic) Apply(input *tactic.Goal, result *tactic.TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if input == nil || result == nil || input.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		if ok, err := input.IsWellSorted(); err != nil {
			return err
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	if err := tactic.FailIfProofGeneration("propagate-ineqs", input); err != nil {
		return err
	}
	if err := tactic.FailIfUnsatCoreGeneration("propagate-ineqs", input); err != nil {
		return err
	}
	if err := result.Prepare(t.manager); err != nil {
		return err
	}
	if err := t.drainGoals(); err != nil {
		return err
	}
	goal, err := t.core.Apply(input)
	if err == nil && ast.ASTSourceDebug {
		var ok bool
		ok, err = goal.IsWellSorted()
		if err == nil && !ok {
			err = ast.ErrASTDomain
		}
	}
	if err == nil {
		if goal == input {
			return result.AddGoal(goal)
		}
		err = result.TakeGoal(goal)
	}
	if err != nil && goal != nil && goal != input {
		t.pending = append(t.pending, goal)
		err = errors.Join(err, t.drainGoals())
	}
	return err
}

func (t *PropagateIneqsTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drainGoals(); err != nil {
		return err
	}
	return t.core.Cleanup()
}

func (t *PropagateIneqsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return newPropagateIneqsAdapter(m, t.context, &t.core.params, 47)
}

func (t *PropagateIneqsTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.core.UpdateParams(p)
}

// Source collect_param_descrs and the inherited statistics/hooks are empty.
func (t *PropagateIneqsTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *PropagateIneqsTactic) CollectStatistics() map[string]float64         { return nil }
func (t *PropagateIneqsTactic) ResetStatistics() error                        { return t.open() }
func (t *PropagateIneqsTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *PropagateIneqsTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *PropagateIneqsTactic) Reset() error                                  { return t.Cleanup() }
