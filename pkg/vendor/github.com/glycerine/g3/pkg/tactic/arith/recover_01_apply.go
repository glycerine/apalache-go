package arith

// Source: src/tactic/arith/recover_01_tactic.cpp, imp::operator().

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func recover01Apply(goal *tactic.Goal, rw *rewriter.TheoryRewriter, services model.ModelEvaluatorConfig, maxBits uint32, out *tactic.TacticResult, trace ...*diagnostic.TraceTags) (err error) {
	if goal == nil || rw == nil || out == nil {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		wellSorted, e := goal.IsWellSorted()
		if e != nil {
			return e
		}
		if !wellSorted {
			return ast.ErrASTDomain
		}
	}
	if err = tactic.FailIfProofGeneration("recover-01", goal); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("recover-01", goal); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("recover-01", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	m := goal.Manager()
	if err = out.Prepare(m); err != nil {
		return err
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
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
	groups, err := recover01Clauses(m, roots, maxBits)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return out.AddGoal(goal)
	}
	newGoal, err := tactic.NewGoalConfigCopy(goal)
	if err != nil {
		return err
	}
	taken := false
	defer func() {
		if !taken {
			err = errors.Join(err, newGoal.Close())
		}
	}()
	if err = newGoal.IncDepth(); err != nil {
		return err
	}
	type group struct {
		decl ast.ASTHandle
		id   uint32
	}
	ordered := make([]group, 0, len(groups))
	saved := make(map[ast.ASTHandle]bool)
	for decl, clauses := range groups {
		id, e := m.GetASTID(decl)
		if e != nil {
			return e
		}
		ordered = append(ordered, group{decl, id})
		for _, clause := range clauses {
			saved[clause] = true
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].id < ordered[j].id })
	for _, root := range roots {
		if !saved[root] {
			if err = newGoal.Assert(root); err != nil {
				return err
			}
		}
	}
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	defs := &recover01Definitions{manager: m, goal: newGoal, sub: sub}
	defer func() { err = errors.Join(err, defs.Close()) }()
	var recovered uint32
	for _, group := range ordered {
		clauses := groups[group.decl]
		literals, coeffs, ok, e := recover01Coefficients(m, clauses)
		if e != nil {
			return e
		}
		if !ok {
			for _, clause := range clauses {
				if e = newGoal.Assert(clause); e != nil {
					return e
				}
			}
			continue
		}
		variable, e := m.MkApp(group.decl, nil)
		if e != nil {
			return e
		}
		if e = defs.build(variable, literals, coeffs, func(variable, definition ast.ASTHandle) error {
			return emitRecover01Definition(m, trace, services.Trace, variable, definition)
		}); e != nil {
			return e
		}
		recovered++
	}
	if recovered == 0 {
		return out.AddGoal(goal)
	}
	if err = tactic.ReportTacticProgress(":recovered-01-vars", recovered); err != nil {
		return err
	}
	if err = rw.SetSubstitution(&sub); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rw.SetSubstitution(nil)) }()
	output, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&output)) }()
	size, err = newGoal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		form, e := newGoal.Form(i)
		if e != nil {
			return e
		}
		if e = rw.Rewrite(form, output); e != nil {
			return e
		}
		value, e := m.RefGet(output)
		if e != nil {
			return e
		}
		if e = newGoal.Update(i, value, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return e
		}
	}
	if models {
		if err = defs.publishModel(services, out); err != nil {
			return err
		}
	}
	if err = out.TakeGoal(newGoal); err != nil {
		return err
	}
	taken = true
	if ast.ASTSourceDebug {
		wellSorted, e := newGoal.IsWellSorted()
		if e != nil {
			return e
		}
		if !wellSorted {
			return ast.ErrASTDomain
		}
	}
	return emitRecover01Goal(newGoal, trace, services.Trace)
}
