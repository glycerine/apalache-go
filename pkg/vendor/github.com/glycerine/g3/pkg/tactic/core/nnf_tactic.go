package core

// Source: src/tactic/core/nnf_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	normalforms "github.com/glycerine/g3/pkg/ast/normal_forms"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type NNFTactic struct {
	self       *NNFTactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	params     uparams.ParamsRef
	full       bool
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*NNFTactic)(nil)

func newNNFTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, full bool) (*NNFTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	stored, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(NNFTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&stored))
	}
	t := &NNFTactic{manager: m, context: context, globals: globals, params: stored, full: full, ownerBytes: amount}
	t.self = t
	if params != nil {
		if err = context.Assign(&t.params, params); err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	if err = t.applyMode(); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func NewSNFTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (*NNFTactic, error) {
	return newNNFTactic(m, context, globals, params, false)
}

func NewNNFTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (*NNFTactic, error) {
	return newNNFTactic(m, context, globals, params, true)
}

func (t *NNFTactic) applyMode() error {
	if !t.full {
		return nil
	}
	mode, err := t.manager.Symbols.Intern("full")
	if err != nil {
		return err
	}
	key := "mode"
	return t.context.SetSymbolName(&t.params, &key, mode)
}

func (t *NNFTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *NNFTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *NNFTactic) DecRef() error {
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
func (t *NNFTactic) Close() error {
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
	t.ownerBytes, t.closed = 0, true
	return nil
}
func (t *NNFTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	var err error
	if params == nil {
		err = t.context.Reset(&t.params)
	} else {
		err = t.context.Assign(&t.params, params)
	}
	if err != nil {
		return err
	}
	return t.applyMode()
}
func (t *NNFTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *NNFTactic) CollectStatistics() map[string]float64         { return nil }
func (t *NNFTactic) ResetStatistics() error                        { return t.open() }
func (t *NNFTactic) Cleanup() error                                { return t.open() }
func (t *NNFTactic) Reset() error                                  { return t.open() }
func (t *NNFTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *NNFTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *NNFTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return newNNFTactic(m, t.context, t.globals, &t.params, t.full)
}

func releaseNNFRefs(m *ast.ASTManager, refs *[]ast.ASTRef) (result error) {
	for len(*refs) != 0 {
		i := len(*refs) - 1
		result = errors.Join(result, m.DelRef(&(*refs)[i]))
		*refs = (*refs)[:i]
	}
	return result
}

func (t *NNFTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
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
	report, err := tactic.NewTacticReport("nnf", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	names, err := normalforms.NewDefinedNames(t.manager)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, names.Close()) }()
	nnf, err := normalforms.NewNNFFromParams(t.manager, names, t.context, t.globals, &t.params)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, nnf.Close()) }()
	var definitions, definitionProofs []ast.ASTRef
	defer func() {
		err = errors.Join(err, releaseNNFRefs(t.manager, &definitionProofs), releaseNNFRefs(t.manager, &definitions))
	}()
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proofOut, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proofOut)) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		current, getErr := goal.Form(i)
		if getErr != nil {
			return getErr
		}
		if err = nnf.Transform(current, &definitions, &definitionProofs, out, proofOut); err != nil {
			return err
		}
		value, getErr := t.manager.RefGet(out)
		if getErr != nil {
			return getErr
		}
		var proof ast.ASTHandle
		if proofs {
			oldProof, e := goal.Proof(i)
			if e != nil {
				return e
			}
			rewriteProof, e := t.manager.RefGet(proofOut)
			if e != nil {
				return e
			}
			proof, e = t.manager.MkProofModusPonens(oldProof, rewriteProof)
			if e != nil {
				return e
			}
		}
		dependency, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if err = goal.Update(i, value, proof, dependency); err != nil {
			return err
		}
	}
	for i, definition := range definitions {
		value, getErr := t.manager.RefGet(definition)
		if getErr != nil {
			return getErr
		}
		var proof ast.ASTHandle
		if proofs {
			if i >= len(definitionProofs) {
				return ast.ErrASTDomain
			}
			proof, getErr = t.manager.RefGet(definitionProofs[i])
			if getErr != nil {
				return getErr
			}
		}
		if err = goal.AssertWithProof(value, proof, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	count, err := names.NumNames()
	if err != nil {
		return err
	}
	if models && count != 0 {
		filter, createErr := tactic.NewFilterModel(t.manager)
		if createErr != nil {
			return createErr
		}
		if err = result.SetModelConverter(filter); err != nil {
			return errors.Join(err, filter.Close())
		}
		for i := uint32(0); i < count; i++ {
			declaration, getErr := names.NameDecl(i)
			if getErr != nil {
				return getErr
			}
			if err = filter.Insert(declaration); err != nil {
				return err
			}
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil || !sorted {
		if checkErr != nil {
			return checkErr
		}
		return ast.ErrASTDomain
	}
	return result.AddGoal(goal)
}
