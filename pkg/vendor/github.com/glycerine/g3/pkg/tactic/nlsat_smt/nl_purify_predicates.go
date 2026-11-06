package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, rw_cfg Boolean mode.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// nlPredicateRewriter abstracts arithmetic atoms into Boolean interface
// predicates, adding the polarity-directed clauses to the nonlinear goal.
type nlPredicateRewriter struct {
	manager       *ast.ASTManager
	arithmetic    ast.FamilyID
	real          ast.ASTHandle
	nonlinear     *tactic.Goal
	polarities    map[ast.ASTHandle]polarity
	memo          map[ast.ASTHandle]ast.ASTHandle
	refs          []ast.ASTRef
	newPredicates []ast.ASTHandle
}

func newNLPredicateRewriter(m *ast.ASTManager, nonlinear *tactic.Goal, polarities map[ast.ASTHandle]polarity) (*nlPredicateRewriter, error) {
	if m == nil || nonlinear == nil || nonlinear.Manager() != m || polarities == nil {
		return nil, ast.ErrASTDomain
	}
	if err := nonlinear.CheckOwner(); err != nil {
		return nil, err
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	real, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		return nil, err
	}
	return &nlPredicateRewriter{manager: m, arithmetic: family, real: real, nonlinear: nonlinear, polarities: polarities, memo: make(map[ast.ASTHandle]ast.ASTHandle)}, nil
}

func (r *nlPredicateRewriter) hold(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err == nil {
		r.refs = append(r.refs, ref)
	}
	return err
}

func (r *nlPredicateRewriter) abstract(atom ast.ASTHandle) (ast.ASTHandle, error) {
	pol, ok := r.polarities[atom]
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	boolean, err := r.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pred, err := r.manager.MkFreshConstNullable(nil, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(pred); err != nil {
		return ast.ASTHandle{}, err
	}
	if pol != negative {
		notPred, e := r.manager.MkNot(pred)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.hold(notPred); e != nil {
			return ast.ASTHandle{}, e
		}
		clause, e := r.manager.MkOr(notPred, atom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.nonlinear.AssertWithProof(clause, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return ast.ASTHandle{}, e
		}
	}
	if pol != positive {
		notAtom, e := r.manager.MkNot(atom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.hold(notAtom); e != nil {
			return ast.ASTHandle{}, e
		}
		clause, e := r.manager.MkOr(pred, notAtom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.nonlinear.AssertWithProof(clause, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return ast.ASTHandle{}, e
		}
	}
	r.polarities[pred] = pol
	r.newPredicates = append(r.newPredicates, pred)
	return pred, nil
}

func (r *nlPredicateRewriter) rewrite(term ast.ASTHandle) (ast.ASTHandle, error) {
	if r == nil || r.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if value, ok := r.memo[term]; ok {
		return value, nil
	}
	kind, err := r.manager.GetKind(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, tactic.NewTacticException("quantifiers are not supported in mixed-mode nlsat engine")
	}
	count, err := r.manager.GetNumArgs(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, count)
	changed := false
	for i := range args {
		old, e := r.manager.GetArg(term, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = r.rewrite(old)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || args[i] != old
	}
	atom := term
	if changed {
		decl, e := r.manager.GetDecl(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		atom, e = r.manager.MkApp(decl, args)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.hold(atom); e != nil {
			return ast.ASTHandle{}, e
		}
	}
	family, err := r.manager.GetFamilyID(atom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := r.manager.GetDeclKind(atom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	match := family == r.arithmetic && op >= 2 && op <= 5
	if family == r.manager.BasicFamilyID() && op == ast.ASTOpEq && len(args) > 0 {
		sort, e := r.manager.GetSort(args[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		match = sort == r.real
	}
	value := atom
	if match {
		// The source looks up polarity on the rebuilt application.
		value, err = r.abstract(atom)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if err = r.hold(term); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(value); err != nil {
		return ast.ASTHandle{}, err
	}
	r.memo[term] = value
	return value, nil
}

func (r *nlPredicateRewriter) rewriteGoal(g *tactic.Goal) error {
	if r == nil || r.manager == nil || g == nil || g == r.nonlinear || g.Manager() != r.manager {
		return ast.ErrASTDomain
	}
	if err := g.CheckOwner(); err != nil {
		return err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		return tactic.NewTacticException("qfufnra-purify does not support proof generation")
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		old, e := g.Form(i)
		if e != nil {
			return e
		}
		value, e := r.rewrite(old)
		if e != nil {
			return e
		}
		dependency, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, value, ast.ASTHandle{}, dependency); e != nil {
			return e
		}
	}
	return nil
}

func (r *nlPredicateRewriter) Close() (result error) {
	if r == nil || r.manager == nil {
		return nil
	}
	for i := len(r.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.refs[i]))
	}
	r.manager = nil
	r.refs = nil
	r.memo = nil
	return result
}
