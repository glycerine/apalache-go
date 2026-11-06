package core

// Source: src/tactic/core/distribute_forall_tactic.cpp/.h.

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type distributeForallConfiguration struct {
	rewriter.DefaultConfiguration
	manager *ast.ASTManager
}

func (c *distributeForallConfiguration) operation(h ast.ASTHandle, operation int32) (bool, error) {
	kind, err := c.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := c.manager.GetFamilyID(h)
	if err != nil || family != c.manager.BasicFamilyID() {
		return false, err
	}
	op, err := c.manager.GetDeclKind(h)
	return op == operation, err
}

func (c *distributeForallConfiguration) distribute(q, body ast.ASTHandle, negate bool, out, proof ast.ASTRef) (result error) {
	count, err := c.manager.GetNumArgs(body)
	if err != nil {
		return fmt.Errorf("distribute-forall argument count: %w", err)
	}
	refs := make([]ast.ASTRef, 0, count)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
	}()
	args := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		arg, getErr := c.manager.GetArg(body, i)
		if getErr != nil {
			return fmt.Errorf("distribute-forall argument: %w", getErr)
		}
		if negate {
			arg, getErr = c.manager.MkNot(arg)
			if getErr != nil {
				return getErr
			}
		}
		updated, getErr := c.manager.UpdateQuantifierBody(q, arg)
		if getErr != nil {
			return fmt.Errorf("distribute-forall update quantifier: %w", getErr)
		}
		reduced, getErr := rewriter.ElimUnusedVarsUFBV(c.manager, updated)
		if reduced != (ast.ASTRef{}) {
			refs = append(refs, reduced)
		}
		if getErr != nil {
			return fmt.Errorf("distribute-forall eliminate unused variables: %w", getErr)
		}
		value, getErr := c.manager.RefGet(reduced)
		if getErr != nil {
			return fmt.Errorf("distribute-forall eliminated result: %w", getErr)
		}
		args = append(args, value)
	}
	conjunction, err := c.manager.MkAndArgs(args)
	if err != nil {
		return fmt.Errorf("distribute-forall conjunction: %w", err)
	}
	if err = c.manager.SetRef(out, conjunction); err != nil {
		return fmt.Errorf("distribute-forall result output: %w", err)
	}
	proofs, err := c.manager.ProofsEnabled()
	if err != nil || !proofs {
		if err != nil {
			return fmt.Errorf("distribute-forall proof mode: %w", err)
		}
		return nil
	}
	step, err := c.manager.MkProofRewrite(q, conjunction)
	if err != nil {
		return fmt.Errorf("distribute-forall proof rewrite: %w", err)
	}
	if err = c.manager.SetRef(proof, step); err != nil {
		return fmt.Errorf("distribute-forall proof output: %w", err)
	}
	return nil
}

func (c *distributeForallConfiguration) ReduceQuantifier(q, body ast.ASTHandle, _, _ ast.AppArguments, out, proof ast.ASTRef) (bool, error) {
	forall, err := c.manager.IsForall(q)
	if err != nil || !forall {
		return false, err
	}
	if not, child, getErr := c.manager.IsNot(body); getErr != nil {
		return false, getErr
	} else if not {
		if or, getErr := c.operation(child, ast.ASTOpOr); getErr != nil {
			return false, getErr
		} else if or {
			return true, c.distribute(q, child, true, out, proof)
		}
	}
	and, err := c.operation(body, ast.ASTOpAnd)
	if err != nil || !and {
		return false, err
	}
	return true, c.distribute(q, body, false, out, proof)
}

type DistributeForallTactic struct {
	self       *DistributeForallTactic
	manager    *ast.ASTManager
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*DistributeForallTactic)(nil)

func NewDistributeForallTactic(m *ast.ASTManager, _ *uparams.ParamsRef) (*DistributeForallTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(DistributeForallTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &DistributeForallTactic{manager: m, ownerBytes: amount}
	t.self = t
	return t, nil
}

func (t *DistributeForallTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}

func (t *DistributeForallTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *DistributeForallTactic) DecRef() error {
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

func (t *DistributeForallTactic) Close() error {
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

func (t *DistributeForallTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DistributeForallTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DistributeForallTactic) CollectStatistics() map[string]float64         { return nil }
func (t *DistributeForallTactic) ResetStatistics() error                        { return t.open() }
func (t *DistributeForallTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DistributeForallTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *DistributeForallTactic) Cleanup() error                                { return t.open() }
func (t *DistributeForallTactic) Reset() error                                  { return t.open() }

func (t *DistributeForallTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewDistributeForallTactic(m, nil)
}

func (t *DistributeForallTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
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
	report, err := tactic.NewTacticReport("distribute-forall", goal)
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
	configuration := &distributeForallConfiguration{manager: t.manager}
	var traversal *rewriter.Traversal
	if proofs {
		traversal, err = rewriter.NewProofTraversal(t.manager, configuration)
	} else {
		traversal, err = rewriter.NewTraversal(t.manager, configuration)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, traversal.Close()) }()
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
		if proofs {
			err = traversal.RewriteWithProof(current, out, proof)
		} else {
			err = traversal.Rewrite(current, out)
		}
		if err != nil {
			return fmt.Errorf("distribute-forall traversal: %w", err)
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
