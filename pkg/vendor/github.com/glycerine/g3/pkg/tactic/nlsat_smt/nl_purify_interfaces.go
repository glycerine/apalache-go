package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, rw_cfg interface-var mode.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// nlInterfaceRewriter owns introduced interface constants, definitions, and
// rewritten terms until Close. Definitions are separated by the solver that
// receives them in the source tactic.
type nlInterfaceRewriter struct {
	manager              *ast.ASTManager
	arithmetic           ast.FamilyID
	real                 ast.ASTHandle
	cache                map[ast.ASTHandle]ast.ASTHandle
	memo                 map[ast.ASTHandle]ast.ASTHandle
	refs                 []ast.ASTRef
	newReals             []ast.ASTHandle
	interfaceValues      []ast.ASTHandle
	nonlinearDefinitions []ast.ASTHandle
	smtDefinitions       []ast.ASTHandle
}

func newNLInterfaceRewriter(m *ast.ASTManager) (*nlInterfaceRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	real, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		return nil, err
	}
	return &nlInterfaceRewriter{manager: m, arithmetic: family, real: real, cache: make(map[ast.ASTHandle]ast.ASTHandle), memo: make(map[ast.ASTHandle]ast.ASTHandle)}, nil
}

func (r *nlInterfaceRewriter) hold(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err == nil {
		r.refs = append(r.refs, ref)
	}
	return err
}

func (r *nlInterfaceRewriter) interfaceVar(arg ast.ASTHandle) (ast.ASTHandle, error) {
	if value, ok := r.cache[arg]; ok {
		return value, nil
	}
	family, err := r.manager.GetFamilyID(arg)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := r.manager.GetNumArgs(arg)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family == ast.NullFamilyID && count == 0 {
		if err = r.hold(arg); err != nil {
			return ast.ASTHandle{}, err
		}
		r.cache[arg] = arg
		r.interfaceValues = append(r.interfaceValues, arg)
		return arg, nil
	}
	value, err := r.manager.MkFreshConstNullable(nil, r.real)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(value); err != nil {
		return ast.ASTHandle{}, err
	}
	eq, err := r.manager.MkEq(value, arg)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(eq); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(arg); err != nil {
		return ast.ASTHandle{}, err
	}
	r.cache[arg] = value
	r.interfaceValues = append(r.interfaceValues, value)
	r.newReals = append(r.newReals, value)
	if family == r.arithmetic {
		r.nonlinearDefinitions = append(r.nonlinearDefinitions, eq)
	} else {
		r.smtDefinitions = append(r.smtDefinitions, eq)
	}
	return value, nil
}

func (r *nlInterfaceRewriter) rewrite(term ast.ASTHandle) (ast.ASTHandle, error) {
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
	family, err := r.manager.GetFamilyID(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := r.manager.GetNumArgs(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, count)
	changed := false
	for i := range args {
		original, e := r.manager.GetArg(term, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = r.rewrite(original)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || args[i] != original
	}
	// Numerals have no arguments, so the source's numeral guard needs no
	// additional handling in this bottom-up traversal.
	for i, arg := range args {
		argFamily, e := r.manager.GetFamilyID(arg)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		needs := family == r.arithmetic && argFamily != r.arithmetic
		if family != r.arithmetic {
			sort, e := r.manager.GetSort(arg)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			needs = sort == r.real
		}
		if needs {
			args[i], e = r.interfaceVar(arg)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			changed = changed || args[i] != arg
		}
	}
	value := term
	if changed {
		decl, e := r.manager.GetDecl(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		value, e = r.manager.MkApp(decl, args)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	if err = r.hold(value); err != nil {
		return ast.ASTHandle{}, err
	}
	if value != term {
		if err = r.hold(term); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	r.memo[term] = value
	return value, nil
}

// rewriteGoal performs the source's first goal rewrite and publishes newly
// introduced arithmetic definitions to the nonlinear goal. SMT definitions
// remain available in smtDefinitions for the cooperating solver.
func (r *nlInterfaceRewriter) rewriteGoal(g, nonlinear *tactic.Goal) error {
	if r == nil || r.manager == nil || g == nil || nonlinear == nil || g == nonlinear || g.Manager() != r.manager || nonlinear.Manager() != r.manager {
		return ast.ErrASTDomain
	}
	if err := g.CheckOwner(); err != nil {
		return err
	}
	if err := nonlinear.CheckOwner(); err != nil {
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
		original, err := g.Form(i)
		if err != nil {
			return err
		}
		start := len(r.nonlinearDefinitions)
		value, err := r.rewrite(original)
		if err != nil {
			return err
		}
		for _, definition := range r.nonlinearDefinitions[start:] {
			if err = nonlinear.AssertWithProof(definition, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
				return err
			}
		}
		dependency, err := g.Dependency(i)
		if err != nil {
			return err
		}
		if err = g.Update(i, value, ast.ASTHandle{}, dependency); err != nil {
			return err
		}
	}
	return nil
}

func (r *nlInterfaceRewriter) Close() (result error) {
	if r == nil || r.manager == nil {
		return nil
	}
	for i := len(r.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.refs[i]))
	}
	r.manager = nil
	r.refs = nil
	r.cache = nil
	r.memo = nil
	return result
}
