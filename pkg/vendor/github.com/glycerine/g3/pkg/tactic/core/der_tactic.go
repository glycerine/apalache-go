package core

// Source: src/tactic/core/der_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type DERTactic struct {
	self       *DERTactic
	manager    *ast.ASTManager
	rewriter   *rewriter.DERRewriter
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*DERTactic)(nil)

func NewDERTactic(m *ast.ASTManager) (*DERTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(DERTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &DERTactic{manager: m, ownerBytes: amount}
	t.self = t
	var err error
	t.rewriter, err = rewriter.NewDERRewriter(m)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *DERTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}

func (t *DERTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *DERTactic) DecRef() error {
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

func (t *DERTactic) Close() (result error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.rewriter != nil {
		result = errors.Join(result, t.rewriter.Close())
		t.rewriter = nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return result
}

func (t *DERTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DERTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DERTactic) CollectStatistics() map[string]float64         { return nil }
func (t *DERTactic) ResetStatistics() error                        { return t.open() }
func (t *DERTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DERTactic) SetProgressCallback(func(uint32)) error        { return t.open() }

func (t *DERTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.rewriter.Cleanup()
}

func (t *DERTactic) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.rewriter.Reset()
}

func (t *DERTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewDERTactic(m)
}

func (t *DERTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	report, err := tactic.NewTacticReport("der", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proof, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proof)) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		inconsistent, checkErr := goal.Inconsistent()
		if checkErr != nil {
			return checkErr
		}
		if inconsistent {
			break
		}
		current, getErr := goal.Form(i)
		if getErr != nil {
			return getErr
		}
		if err = t.rewriter.Rewrite(current, out, proof); err != nil {
			return err
		}
		value, getErr := t.manager.RefGet(out)
		if getErr != nil {
			return getErr
		}
		var newProof ast.ASTHandle
		if proofs {
			oldProof, getErr := goal.Proof(i)
			if getErr != nil {
				return getErr
			}
			rewriteProof, getErr := t.manager.RefGet(proof)
			if getErr != nil {
				return getErr
			}
			newProof, getErr = t.manager.MkProofModusPonens(oldProof, rewriteProof)
			if getErr != nil {
				return getErr
			}
		}
		dependency, getErr := goal.Dependency(i)
		if getErr != nil {
			return getErr
		}
		if err = goal.Update(i, value, newProof, dependency); err != nil {
			return err
		}
	}
	if err = goal.ElimRedundancies(); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
