package core

// Source: src/tactic/core/split_clause_tactic.cpp/.h.

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SplitClauseTactic struct {
	self          *SplitClauseTactic
	manager       *ast.ASTManager
	context       *uparams.ParamsContext
	largestClause bool
	references    uint32
	ownerBytes    uint64
	closed        bool
}

var _ tactic.Tactic = (*SplitClauseTactic)(nil)

func NewSplitClauseTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (*SplitClauseTactic, error) {
	if m == nil || c == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(SplitClauseTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &SplitClauseTactic{manager: m, context: c, ownerBytes: amount}
	t.self = t
	if err := t.readParams(p); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeSplitClauseTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (tactic.Tactic, error) {
	child, err := NewSplitClauseTactic(m, c, p)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return clean, nil
}

func (t *SplitClauseTactic) open() error {
	if t == nil || t.self != t || t.manager == nil || t.context == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}

func (t *SplitClauseTactic) readParams(p *uparams.ParamsRef) error {
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	name := "split_largest_clause"
	value, err := t.context.GetBoolName(p, &name, false)
	if err == nil {
		t.largestClause = value
	}
	return err
}

func (t *SplitClauseTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}

func (t *SplitClauseTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references == 0 {
		return t.Close()
	}
	return nil
}

func (t *SplitClauseTactic) Close() error {
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

func (t *SplitClauseTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.readParams(p)
}

func (t *SplitClauseTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	description := "(default: false) split the largest clause in the goal."
	return out.InsertName("split_largest_clause", uparams.ParamBool, &description, nil, nil)
}

func (t *SplitClauseTactic) CollectStatistics() map[string]float64  { return nil }
func (t *SplitClauseTactic) ResetStatistics() error                 { return t.open() }
func (t *SplitClauseTactic) Cleanup() error                         { return t.open() }
func (t *SplitClauseTactic) Reset() error                           { return t.Cleanup() }
func (t *SplitClauseTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *SplitClauseTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *SplitClauseTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	translated, err := NewSplitClauseTactic(m, t.context, nil)
	if err == nil {
		translated.largestClause = t.largestClause
	}
	return translated, err
}

func (t *SplitClauseTactic) selectClause(goal *tactic.Goal) (uint32, bool, error) {
	size, err := goal.Size()
	if err != nil {
		return 0, false, err
	}
	var selected, length uint32
	found := false
	for i := uint32(0); i < size; i++ {
		formula, getErr := goal.Form(i)
		if getErr != nil {
			return 0, false, getErr
		}
		kind, getErr := t.manager.GetKind(formula)
		if getErr != nil {
			return 0, false, getErr
		}
		if kind != ast.ASTApp {
			continue
		}
		family, getErr := t.manager.GetFamilyID(formula)
		if getErr != nil {
			return 0, false, getErr
		}
		operation, getErr := t.manager.GetDeclKind(formula)
		if getErr != nil {
			return 0, false, getErr
		}
		if family != t.manager.BasicFamilyID() || operation != ast.ASTOpOr {
			continue
		}
		count, getErr := t.manager.GetNumArgs(formula)
		if getErr != nil {
			return 0, false, getErr
		}
		if count < 2 {
			continue
		}
		if !t.largestClause {
			return i, true, nil
		}
		if count > length {
			selected, length, found = i, count, true
		}
	}
	return selected, found, nil
}

func (t *SplitClauseTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || out == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if sorted, err := goal.IsWellSorted(); err != nil {
		return err
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("split-clause", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { resultErr = errors.Join(resultErr, report.Close()) }()
	}
	position, found, err := t.selectClause(goal)
	if err != nil {
		return err
	}
	if !found {
		return tactic.NewTacticException("split-clause tactic failed, goal does not contain any clause")
	}
	clause, err := goal.Form(position)
	if err != nil {
		return err
	}
	dependency, err := goal.Dependency(position)
	if err != nil {
		return err
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		proof, getErr := goal.Proof(position)
		if getErr != nil {
			return getErr
		}
		converter, getErr := newSplitClauseProofConverter(t.manager, clause, proof)
		if getErr != nil {
			return getErr
		}
		if getErr = out.SetProofConverter(converter); getErr != nil {
			return errors.Join(getErr, converter.Close())
		}
	}
	count, err := t.manager.GetNumArgs(clause)
	if err != nil {
		return err
	}
	if err = tactic.ReportTacticProgress(":num-new-branches", count); err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		subgoal := goal
		owned := false
		if i+1 != count {
			subgoal, err = tactic.NewGoalCopy(goal)
			if err != nil {
				return err
			}
			owned = true
		}
		literal, getErr := t.manager.GetArg(clause, i)
		if getErr != nil {
			if owned {
				getErr = errors.Join(getErr, subgoal.Close())
			}
			return getErr
		}
		var proof ast.ASTHandle
		if proofs {
			proof, getErr = t.manager.MkProofHypothesis(literal)
			if getErr != nil {
				if owned {
					getErr = errors.Join(getErr, subgoal.Close())
				}
				return getErr
			}
		}
		if getErr = subgoal.Update(position, literal, proof, dependency); getErr != nil {
			if owned {
				getErr = errors.Join(getErr, subgoal.Close())
			}
			return getErr
		}
		if getErr = subgoal.IncDepth(); getErr != nil {
			if owned {
				getErr = errors.Join(getErr, subgoal.Close())
			}
			return getErr
		}
		if owned {
			getErr = out.TakeGoal(subgoal)
		} else {
			getErr = out.AddGoal(subgoal)
		}
		if getErr != nil {
			if owned {
				getErr = errors.Join(getErr, subgoal.Close())
			}
			return getErr
		}
	}
	return nil
}

type splitClauseProofConverter struct {
	self        *splitClauseProofConverter
	manager     *ast.ASTManager
	clause      ast.ASTRef
	clauseProof ast.ASTRef
	references  uint32
	ownerBytes  uint64
	closed      bool
}

var _ tactic.ProofConverter = (*splitClauseProofConverter)(nil)

func newSplitClauseProofConverter(m *ast.ASTManager, clause, proof ast.ASTHandle) (_ *splitClauseProofConverter, resultErr error) {
	if m == nil || clause.IsNull() || proof.IsNull() {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(splitClauseProofConverter{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	c := &splitClauseProofConverter{manager: m, ownerBytes: amount}
	c.self = c
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.Close())
		}
	}()
	var err error
	c.clause, err = m.NewRef(clause)
	if err != nil {
		return nil, err
	}
	c.clauseProof, err = m.NewRef(proof)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *splitClauseProofConverter) open() error {
	if c == nil || c.self != c || c.manager == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

func (c *splitClauseProofConverter) IncRef() error {
	if err := c.open(); err != nil {
		return err
	}
	c.references++
	return nil
}

func (c *splitClauseProofConverter) DecRef() error {
	if err := c.open(); err != nil {
		return err
	}
	if c.references == 0 {
		return arena.ErrBounds
	}
	c.references--
	if c.references == 0 {
		return c.Close()
	}
	return nil
}

func (c *splitClauseProofConverter) Convert(m *ast.ASTManager, sources []ast.ASTHandle, out ast.ASTRef) error {
	if err := c.open(); err != nil {
		return err
	}
	if m != c.manager {
		return ast.ErrASTHandle
	}
	clause, err := m.RefGet(c.clause)
	if err != nil {
		return err
	}
	count, err := m.GetNumArgs(clause)
	if err != nil {
		return err
	}
	if uint32(len(sources)) != count {
		return ast.ErrASTDomain
	}
	clauseProof, err := m.RefGet(c.clauseProof)
	if err != nil {
		return err
	}
	proofs := make([]ast.ASTHandle, 1, count+1)
	proofs[0] = clauseProof
	for i, source := range sources {
		literal, getErr := m.GetArg(clause, uint32(i))
		if getErr != nil {
			return getErr
		}
		negated, getErr := m.MkNot(literal)
		if getErr != nil {
			return getErr
		}
		lemma, getErr := m.MkProofLemma(source, negated)
		if getErr != nil {
			return getErr
		}
		proofs = append(proofs, lemma)
	}
	result, err := m.MkProofUnitResolution(proofs)
	if err != nil {
		return err
	}
	return m.SetRef(out, result)
}

func (c *splitClauseProofConverter) Translate(translation *ast.ASTTranslation) (tactic.ProofConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != c.manager {
		return nil, ast.ErrASTHandle
	}
	clause, err := c.manager.RefGet(c.clause)
	if err != nil {
		return nil, err
	}
	proof, err := c.manager.RefGet(c.clauseProof)
	if err != nil {
		return nil, err
	}
	clause, err = translation.Translate(clause)
	if err != nil {
		return nil, err
	}
	proof, err = translation.Translate(proof)
	if err != nil {
		return nil, err
	}
	return newSplitClauseProofConverter(translation.To(), clause, proof)
}

func (c *splitClauseProofConverter) Cancel() error { return c.open() }

func (c *splitClauseProofConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	clause, err := c.manager.RefGet(c.clause)
	if err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(c.manager)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "(split-clause "); err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, clause, nil); err != nil {
		return err
	}
	_, err = fmt.Fprint(out, ")")
	return err
}

func (c *splitClauseProofConverter) Close() (result error) {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if c.clause != (ast.ASTRef{}) {
		result = errors.Join(result, c.manager.DelRef(&c.clause))
	}
	if c.clauseProof != (ast.ASTRef{}) {
		result = errors.Join(result, c.manager.DelRef(&c.clauseProof))
	}
	c.manager.Budget().Release(c.ownerBytes)
	c.ownerBytes = 0
	c.closed = true
	return result
}
