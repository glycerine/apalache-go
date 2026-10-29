package core

// Source: src/tactic/core/symmetry_reduce_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SymmetryReduceTactic struct {
	self          *SymmetryReduceTactic
	manager       *ast.ASTManager
	context       *uparams.ParamsContext
	globals       *uparams.GlobalParamsService
	configuration symmetryRewriteConfig
	rewriter      *rewriter.Traversal
	replacer      *rewriter.DefaultExprReplacer
	trace         *diagnostic.TraceTags
	ownerBytes    uint64
	refs          uint32
	closed        bool
}

var _ tactic.Tactic = (*SymmetryReduceTactic)(nil)

// NewSymmetryReduceTactic returns the uncounted source owner. Parameters have
// no algorithmic consumer; the borrowed services are used by mk_pp diagnostics.
func NewSymmetryReduceTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*SymmetryReduceTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(SymmetryReduceTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &SymmetryReduceTactic{manager: m, context: c, globals: g, ownerBytes: bytes, configuration: symmetryRewriteConfig{manager: m}}
	t.self = t
	var err error
	t.rewriter, err = rewriter.NewTraversal(m, &t.configuration)
	if err == nil {
		t.replacer, err = rewriter.NewDefaultExprReplacer(m)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *SymmetryReduceTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *SymmetryReduceTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *SymmetryReduceTactic) DecRef() error {
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
func (t *SymmetryReduceTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.replacer != nil {
		if err := t.replacer.Close(); err != nil {
			return err
		}
		t.replacer = nil
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *SymmetryReduceTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *SymmetryReduceTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *SymmetryReduceTactic) CollectStatistics() map[string]float64         { return nil }
func (t *SymmetryReduceTactic) ResetStatistics() error                        { return t.open() }
func (t *SymmetryReduceTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *SymmetryReduceTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *SymmetryReduceTactic) Cleanup() error                                { return t.open() }
func (t *SymmetryReduceTactic) Reset() error                                  { return t.open() }
func (t *SymmetryReduceTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewSymmetryReduceTactic(m, t.context, t.globals)
	if err == nil {
		other.trace = t.trace
	}
	return other, err
}
func (t *SymmetryReduceTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err := tactic.FailIfProofGeneration("symmetry_reduce", goal); err != nil {
		return err
	}
	if err := tactic.FailIfUnsatCoreGeneration("symmetry_reduce", goal); err != nil {
		return err
	}
	if err := result.Prepare(t.manager); err != nil {
		return err
	}
	if err := t.runSymmetry(goal); err != nil {
		return err
	}
	if err := goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *SymmetryReduceTactic) runSymmetry(goal *tactic.Goal) (err error) {
	inconsistent, err := goal.Inconsistent()
	if err != nil || inconsistent {
		return err
	}
	report, err := tactic.NewTacticReport("symmetry-reduce", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	m := t.manager
	saved := solveEquationFrame{manager: m}
	defer func() { err = errors.Join(err, saved.Close()) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	conjs := make([]ast.ASTHandle, size)
	for i := range conjs {
		conjs[i], err = goal.Form(uint32(i))
		if err != nil {
			return err
		}
	}
	fml, err := m.MkAndArgs(conjs)
	if err != nil {
		return err
	}
	if err = saved.pin(fml); err != nil {
		return err
	}
	fml, err = t.normalize(fml, &saved)
	if err != nil {
		return err
	}
	graph, err := t.symmetryGraph(fml)
	if err != nil {
		return err
	}
	permutations, err := t.candidatePermutations(fml, graph)
	if err != nil || len(permutations) == 0 {
		return err
	}
	var count uint32
	for _, constants := range permutations {
		ok, e := t.checkPermutation(fml, constants, false, &saved)
		if e == nil && ok {
			ok, e = t.checkPermutation(fml, constants, true, &saved)
		}
		if e != nil {
			return e
		}
		if err = t.traceSymmetry(constants, ok); err != nil {
			return err
		}
		if !ok {
			continue
		}
		terms, e := t.selectSymmetryTerms(fml, constants)
		if e != nil {
			return e
		}
		var used []ast.ASTHandle
		for len(terms) != 0 && len(used) < len(constants) {
			index, e := t.promisingSymmetryTerm(terms, used, constants, graph.occurrences)
			if e != nil {
				return e
			}
			term := terms[index]
			terms = append(terms[:index], terms[index+1:]...)
			used, e = t.symmetryUsed(term, used, constants)
			if e != nil {
				return e
			}
			var next ast.ASTHandle
			for _, c := range constants {
				if !symmetryContains(used, c) {
					next = c
					break
				}
			}
			if next.IsNull() {
				break
			}
			used = append(used, next)
			equalities := make([]ast.ASTHandle, len(used))
			for i, c := range used {
				equalities[i], err = m.MkEq(term, c)
				if err != nil {
					return err
				}
				if err = saved.pin(equalities[i]); err != nil {
					return err
				}
			}
			member, e := m.MkOrArgs(equalities)
			if e != nil {
				return e
			}
			if err = saved.pin(member); err != nil {
				return err
			}
			if err = goal.Assert(member); err != nil {
				return err
			}
			count++
			if err = t.traceMember(member); err != nil {
				return err
			}
			fml, err = m.MkAnd(fml, member)
			if err != nil {
				return err
			}
			if err = saved.pin(fml); err != nil {
				return err
			}
			fml, err = t.normalize(fml, &saved)
			if err != nil {
				return err
			}
		}
	}
	return tactic.ReportTacticProgress(":num-symmetry-breaking ", count)
}
