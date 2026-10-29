package arith

// Source: src/tactic/arith/lia2pb_tactic.cpp imp::operator().

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *LIA2PBTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		if ok, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	if err = tactic.FailIfProofGeneration("lia2pb", goal); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("lia2pb", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.bounds.Reset(); err != nil {
		return err
	}
	if err = t.rewriter.Reset(); err != nil {
		return err
	}
	if err = t.resetNewDependencies(); err != nil {
		return err
	}
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return err
	}
	if inconsistent {
		return result.AddGoal(goal)
	}
	if err = t.bounds.CollectGoal(goal); err != nil {
		return err
	}
	if err = t.emitLIA2PB("operator()", 211, t.bounds.Display); err != nil {
		return err
	}
	found := false
	for _, variable := range t.bounds.variables {
		u, ok, e := t.target(variable)
		if e != nil {
			return e
		}
		if ok && u.Cmp(big.NewRat(1, 1)) > 0 {
			found = true
			break
		}
	}
	if !found {
		if err = goal.IncDepth(); err != nil {
			return err
		}
		return result.AddGoal(goal)
	}
	accepted, err := t.check(goal)
	if err != nil {
		return err
	}
	if !accepted {
		return tactic.NewTacticException("goal is in a fragment unsupported by lia2pb")
	}
	var bits uint32
	for _, variable := range t.bounds.variables {
		u, ok, e := t.target(variable)
		if e != nil {
			return e
		}
		if ok && u.Cmp(big.NewRat(1, 1)) > 0 {
			n, e := lia2pbBits(&u)
			if e != nil {
				return e
			}
			bits += n
			if bits > t.totalBits {
				return tactic.NewTacticException("lia2pb failed, number of necessary bits exceeds specified threshold (use option :lia2pb-total-bits to increase threshold)")
			}
		}
	}
	if err = t.convert(goal, result); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if err = t.emitLIA2PB("operator()", 303, goal.Display); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		if ok, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (t *LIA2PBTactic) convert(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	m := t.manager
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	var extension *tactic.ExtensionModelConverter
	var filter *tactic.FilterModelConverter
	if models {
		extension, err = tactic.NewExtensionModel(m, t.services)
		if err != nil {
			return err
		}
		if err = extension.IncRef(); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, extension.DecRef()) }()
		filter, err = tactic.NewFilterModel(m)
		if err != nil {
			return err
		}
		if err = filter.IncRef(); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, filter.DecRef()) }()
		if err = filter.BindTrace(t.trace, t.globals); err != nil {
			return err
		}
		converter, e := tactic.ConcatModels(m.Budget(), filter, extension)
		if e != nil {
			return e
		}
		if err = result.SetModelConverter(converter); err != nil {
			return errors.Join(err, converter.Close())
		}
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	zref, err := m.NewRef(zero)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&zref)) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		return err
	}
	oref, err := m.NewRef(one)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&oref)) }()
	sub, err := m.NewExprSubstitutionPolicy(cores, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	var count uint32
	for _, variable := range t.bounds.variables {
		u, ok, e := t.target(variable)
		if e != nil {
			return e
		}
		if !ok || u.Cmp(big.NewRat(1, 1)) <= 0 {
			continue
		}
		count++
		if err = t.convertVariable(goal, variable, &u, zero, one, sub, extension, filter, cores); err != nil {
			return err
		}
	}
	if err = tactic.ReportTacticProgress(":converted-lia2pb", count); err != nil {
		return err
	}
	if err = t.rewriter.SetSubstitution(&sub); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.rewriter.SetSubstitution(nil)) }()
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&proof)) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		term, e := goal.Form(i)
		if e != nil {
			return e
		}
		proofs, e := m.ProofsEnabled()
		if e != nil {
			return e
		}
		if proofs {
			err = t.rewriter.RewriteWithProof(term, out, proof)
		} else {
			err = t.rewriter.Rewrite(term, out)
		}
		if err != nil {
			return err
		}
		var dep ast.ASTDependency
		if cores {
			used, e := t.rewriter.UsedDependencies()
			if e != nil {
				return e
			}
			old, e := goal.Dependency(i)
			if e != nil {
				return e
			}
			dep, err = m.DependencyJoin(used, old)
			if err != nil {
				return err
			}
			// Protect the join while the rewriter drops its used-dependency root.
			if err = m.DependencyInc(dep); err != nil {
				return err
			}
		}
		err = func() (err error) {
			if cores {
				defer func() { err = errors.Join(err, m.DependencyDec(dep)) }()
				if err = t.rewriter.ResetUsedDependencies(); err != nil {
					return err
				}
			}
			value, err := m.RefGet(out)
			if err != nil {
				return err
			}
			var pr ast.ASTHandle
			if proofs {
				pr, err = m.RefGet(proof)
				if err != nil {
					return err
				}
				old, e := goal.Proof(i)
				if e != nil {
					return e
				}
				pr, err = m.MkProofModusPonens(old, pr)
				if err != nil {
					return err
				}
				if err = m.SetRef(proof, pr); err != nil {
					return err
				}
			}
			return goal.Update(i, value, pr, dep)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *LIA2PBTactic) convertVariable(goal *tactic.Goal, variable ast.ASTHandle, upper *big.Rat, zero, one ast.ASTHandle, sub ast.ASTExprSubstitution, extension *tactic.ExtensionModelConverter, filter *tactic.FilterModelConverter, cores bool) (err error) {
	m := t.manager
	bits, err := lia2pbBits(upper)
	if err != nil {
		return err
	}
	integer, err := m.MkFamilySort(t.bounds.family, 1, nil)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 0, bits)
	var held []ast.ASTRef
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&held[i]))
		}
	}()
	weight := big.NewRat(1, 1)
	for i := uint32(0); i < bits; i++ {
		fresh, e := m.MkFreshConstNullable(nil, integer)
		if e != nil {
			return e
		}
		lower, e := m.MkFamilyApp(t.bounds.family, 2, nil, []ast.ASTHandle{zero, fresh})
		if e != nil {
			return e
		}
		if err = goal.Assert(lower); err != nil {
			return err
		}
		bound, e := m.MkFamilyApp(t.bounds.family, 2, nil, []ast.ASTHandle{fresh, one})
		if e != nil {
			return e
		}
		if err = goal.Assert(bound); err != nil {
			return err
		}
		term := fresh
		if i != 0 {
			n, e := m.ArithmeticNumeralRat(weight, true)
			if e != nil {
				return e
			}
			term, err = m.MkFamilyApp(t.bounds.family, 9, nil, []ast.ASTHandle{n, fresh})
			if err != nil {
				return err
			}
		}
		ref, e := m.NewRef(term)
		if e != nil {
			return e
		}
		held = append(held, ref)
		args = append(args, term)
		if filter != nil {
			decl, e := m.GetDecl(fresh)
			if e != nil {
				return e
			}
			if err = filter.Insert(decl); err != nil {
				return err
			}
		}
		weight.Mul(weight, big.NewRat(2, 1))
	}
	if len(args) < 2 {
		return ast.ErrASTDomain
	}
	definition, err := m.MkFamilyApp(t.bounds.family, 6, nil, args)
	if err != nil {
		return err
	}
	dref, err := m.NewRef(definition)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&dref)) }()
	var dep ast.ASTDependency
	if cores {
		lower, e := t.bounds.LowerDependency(variable)
		if e != nil {
			return e
		}
		upper, e := t.bounds.UpperDependency(variable)
		if e != nil {
			return e
		}
		dep, err = m.DependencyJoin(lower, upper)
		if err != nil {
			return err
		}
		if !dep.IsNull() {
			if err = m.DependencyInc(dep); err != nil {
				return err
			}
			t.newDeps = append(t.newDeps, dep)
		}
	}
	if err = t.traceDefinition(variable, dep); err != nil {
		return err
	}
	if err = m.ExprSubstitutionInsert(sub, variable, definition, ast.ASTHandle{}, dep); err != nil {
		return err
	}
	if extension != nil {
		decl, e := m.GetDecl(variable)
		if e != nil {
			return e
		}
		return extension.Insert(decl, definition)
	}
	return nil
}
