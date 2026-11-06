package ufbv

// Source: src/tactic/ufbv/macro_finder_tactic.cpp/.h and
// src/tactic/ufbv/quasi_macros_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type macroTacticKind uint8

const (
	macroFinderKind macroTacticKind = iota
	quasiMacrosKind
)

// MacroTactic adapts the retained macro algorithms to the tactic goal and
// model-converter boundary. Each application owns a fresh macro manager, as in
// the source tactic implementations.
type MacroTactic struct {
	self       *MacroTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	services   model.ModelEvaluatorConfig
	params     uparams.ParamsRef
	kind       macroTacticKind
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*MacroTactic)(nil)

func newMacroTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, services model.ModelEvaluatorConfig, kind macroTacticKind) (*MacroTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	stored, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(MacroTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&stored))
	}
	t := &MacroTactic{manager: m, context: context, globals: globals, services: services, params: stored, kind: kind, ownerBytes: amount}
	t.self = t
	if params != nil {
		if err = context.Assign(&t.params, params); err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	return t, nil
}

func NewMacroFinderTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*MacroTactic, error) {
	return newMacroTactic(m, context, globals, params, services, macroFinderKind)
}

func NewQuasiMacrosTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*MacroTactic, error) {
	return newMacroTactic(m, context, globals, params, services, quasiMacrosKind)
}

func (t *MacroTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *MacroTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *MacroTactic) DecRef() error {
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

func (t *MacroTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.context.Del(&t.params); err != nil {
		return err
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *MacroTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if params == nil {
		return t.context.Reset(&t.params)
	}
	return t.context.Assign(&t.params, params)
}

func (t *MacroTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil || t.kind != macroFinderKind {
		return nil
	}
	description := "(default: false) eliminate conjunctions during internal simplifier calls."
	return out.InsertName("elim_and", uparams.ParamBool, &description, nil, nil)
}

func (t *MacroTactic) CollectStatistics() map[string]float64 { return nil }
func (t *MacroTactic) ResetStatistics() error                { return t.open() }
func (t *MacroTactic) SetLogic(usymbol.Symbol) error         { return t.open() }
func (t *MacroTactic) SetProgressCallback(func(uint32)) error {
	return t.open()
}
func (t *MacroTactic) Cleanup() error { return t.open() }
func (t *MacroTactic) Reset() error   { return t.open() }

func (t *MacroTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return newMacroTactic(m, t.context, t.globals, &t.params, t.services, t.kind)
}

func macroGoalRows(goal *tactic.Goal) ([]macros.Formula, error) {
	size, err := goal.Size()
	if err != nil {
		return nil, err
	}
	rows := make([]macros.Formula, size)
	for i := uint32(0); i < size; i++ {
		if rows[i].Expression, err = goal.Form(i); err != nil {
			return nil, err
		}
		if rows[i].Proof, err = goal.Proof(i); err != nil {
			return nil, err
		}
		if rows[i].Dependency, err = goal.Dependency(i); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func macroBatchRows(batch *macros.FormulaBatch) ([]macros.Formula, error) {
	rows := make([]macros.Formula, batch.Len())
	for i := range rows {
		row, err := batch.At(i)
		if err != nil {
			return nil, err
		}
		rows[i] = row
	}
	return rows, nil
}

func (t *MacroTactic) run(manager *macros.Manager, inputs []macros.Formula) (*macros.FormulaBatch, error) {
	if t.kind == macroFinderKind {
		finder, err := macros.NewFinder(t.manager, manager)
		if err != nil {
			return nil, err
		}
		return finder.Apply(inputs)
	}
	quasi, err := macros.NewQuasiMacros(t.manager, manager)
	if err != nil {
		return nil, err
	}
	current, more, err := quasi.Apply(inputs)
	if err != nil {
		return nil, err
	}
	for more {
		if canceled, checkErr := t.manager.Canceled(); checkErr != nil {
			return nil, errors.Join(checkErr, current.Close())
		} else if canceled {
			return nil, errors.Join(tactic.NewTacticException(t.manager.Limit.CancelMessage()), current.Close())
		}
		rows, rowsErr := macroBatchRows(current)
		if rowsErr != nil {
			return nil, errors.Join(rowsErr, current.Close())
		}
		next, changed, applyErr := quasi.Apply(rows)
		if applyErr != nil {
			return nil, errors.Join(applyErr, current.Close())
		}
		if closeErr := current.Close(); closeErr != nil {
			return nil, errors.Join(closeErr, next.Close())
		}
		current, more = next, changed
	}
	return current, nil
}

func (t *MacroTactic) publishModelConverter(manager *macros.Manager, result *tactic.TacticResult) (err error) {
	converter, err := tactic.NewExtensionModel(t.manager, t.services)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(converter); err != nil {
		return errors.Join(err, converter.Close())
	}
	definition, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&definition)) }()
	for i := 0; i < manager.NumMacros(); i++ {
		declaration, getErr := manager.MacroInterpretation(i, definition)
		if getErr != nil {
			return getErr
		}
		value, getErr := t.manager.RefGet(definition)
		if getErr != nil {
			return getErr
		}
		if getErr = converter.Insert(declaration, value); getErr != nil {
			return getErr
		}
	}
	return nil
}

func (t *MacroTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	name := "macro-finder"
	if t.kind == quasiMacrosKind {
		name = "quasi-macros"
	}
	report, err := tactic.NewTacticReport(name, goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	inputs, err := macroGoalRows(goal)
	if err != nil {
		return err
	}
	manager, err := macros.NewManager(t.manager)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, manager.Close()) }()
	output, err := t.run(manager, inputs)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	if models {
		if err = t.publishModelConverter(manager, result); err != nil {
			return err
		}
	}
	if err = goal.Reset(); err != nil {
		return err
	}
	for i := 0; i < output.Len(); i++ {
		row, getErr := output.At(i)
		if getErr != nil {
			return getErr
		}
		if !proofs {
			row.Proof = ast.ASTHandle{}
		}
		if !cores {
			row.Dependency = ast.ASTDependency{}
		}
		if err = goal.AssertWithProof(row.Expression, row.Proof, row.Dependency); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	return result.AddGoal(goal)
}
