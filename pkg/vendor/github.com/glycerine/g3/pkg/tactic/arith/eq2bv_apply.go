package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, goal transformation.

import (
	"errors"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func eq2bvIsBound(plan *eq2bvReplacements, form ast.ASTHandle) (bool, error) {
	m := plan.manager
	kind, err := m.GetKind(form)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(form)
	if err != nil {
		return false, err
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	op, err := m.GetDeclKind(form)
	if err != nil || family != arith || (op != 2 && op != 3) {
		return false, err
	}
	left, err := m.GetArg(form, 0)
	if err != nil {
		return false, err
	}
	right, err := m.GetArg(form, 1)
	if err != nil {
		return false, err
	}
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		if plan.nonfd[pair[0]] {
			continue
		}
		// The source also accepts variables seen only in bounds, but no
		// replacement bounds are generated for them. Dropping those formulas
		// can change an unsatisfiable goal into a satisfiable one.
		converted := false
		for _, entry := range plan.entries {
			if entry.original == pair[0] {
				converted = true
				break
			}
		}
		if !converted {
			continue
		}
		constant, e := arithmeticUninterpretedConstant(m, pair[0])
		if e != nil {
			return false, e
		}
		if !constant {
			continue
		}
		text, _, numeral, e := m.ArithmeticNumeralText(pair[1])
		if e != nil {
			return false, e
		}
		if !numeral {
			continue
		}
		value, ok := new(big.Rat).SetString(text)
		if ok && value.IsInt() && value.Sign() >= 0 && value.Num().BitLen() <= 32 {
			return true, nil
		}
	}
	return false, nil
}

func eq2bvApply(goal *tactic.Goal, bounds *BoundManager, result *tactic.TacticResult, trace ...*diagnostic.TraceTags) (err error) {
	if goal == nil || bounds == nil || result == nil || bounds.Manager() != goal.Manager() {
		return ast.ErrASTHandle
	}
	m := goal.Manager()
	if err = result.Prepare(m); err != nil {
		return err
	}
	if err = bounds.Reset(); err != nil {
		return err
	}
	if err = bounds.CollectGoal(goal); err != nil {
		return err
	}
	n, err := goal.Size()
	if err != nil {
		return err
	}
	roots := make([]ast.ASTHandle, n)
	for i := range roots {
		roots[i], err = goal.Form(uint32(i))
		if err != nil {
			return err
		}
	}
	plan, err := eq2bvPrepare(m, bounds, roots)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, plan.Close()) }()
	if len(plan.entries) == 0 {
		return result.AddGoal(goal)
	}
	converter, err := tactic.NewEq2BVModelConverter(m)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, converter.Close())
		}
	}()
	for _, entry := range plan.entries {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return e
		}
		freshDecl, e := m.GetDecl(fresh)
		if e != nil {
			return e
		}
		originalDecl, e := m.GetDecl(entry.original)
		if e != nil {
			return e
		}
		if entry.negativeSentinel {
			e = converter.InsertWithNegativeSentinel(freshDecl, originalDecl, entry.width)
		} else {
			e = converter.Insert(freshDecl, originalDecl)
		}
		if e != nil {
			return e
		}
	}
	boundTerms, err := eq2bvMakeBounds(plan, bounds)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, boundTerms.Close()) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	for i, original := range roots {
		isBound, e := eq2bvIsBound(plan, original)
		if e != nil {
			return e
		}
		if isBound {
			truth, e := m.MkTrue()
			if e != nil {
				return e
			}
			if e = goal.Update(uint32(i), truth, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
				return e
			}
			continue
		}
		output, e := eq2bvRewrite(plan, original)
		if e != nil {
			return e
		}
		value, e := m.RefGet(output)
		if e != nil {
			return errors.Join(e, m.DelRef(&output))
		}
		var proof ast.ASTHandle
		if proofs {
			rewrite, e := m.MkProofRewrite(original, value)
			if e == nil {
				var previous ast.ASTHandle
				previous, e = goal.Proof(uint32(i))
				if e == nil {
					proof, e = m.MkProofModusPonens(previous, rewrite)
				}
			}
			if e != nil {
				return errors.Join(e, m.DelRef(&output))
			}
		}
		dep, e := goal.Dependency(uint32(i))
		if e == nil {
			e = goal.Update(uint32(i), value, proof, dep)
		}
		if e = errors.Join(e, m.DelRef(&output)); e != nil {
			return e
		}
	}
	for _, entry := range boundTerms.entries {
		h, e := m.RefGet(entry.formula)
		if e != nil {
			return e
		}
		var proof ast.ASTHandle
		if proofs {
			proof, e = m.MkProofAsserted(h)
			if e != nil {
				return e
			}
		}
		if e = goal.AssertWithProof(h, proof, entry.dependency); e != nil {
			return e
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.SetModelConverter(converter); err != nil {
		return err
	}
	published = true
	if err = result.AddGoal(goal); err != nil {
		return err
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
	tags := m.BorrowSourceTraceTags()
	if len(trace) != 0 && trace[0] != nil {
		tags = trace[0]
	}
	if !tags.ShouldTrace("pb") {
		return nil
	}
	err = tags.EmitConditional("pb", "operator()", "src/tactic/arith/eq2bv_tactic.cpp", 210,
		func() bool { return true }, func(out io.Writer) error {
			return goal.Display(fmTraceOstream{out})
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
