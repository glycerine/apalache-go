package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, get_unsat_core,
// core2result and the unsatisfiable SMT branch of solve.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
)

type nlCoreTracker struct {
	manager    *ast.ASTManager
	extraction *solver.ClauseExtraction
	context    map[ast.ASTHandle]struct{}
	usedSet    map[ast.ASTHandle]struct{}
	used       []ast.ASTHandle
}

func newNLCoreTracker(m *ast.ASTManager, extraction *solver.ClauseExtraction) (*nlCoreTracker, error) {
	if m == nil || extraction == nil {
		return nil, ast.ErrASTHandle
	}
	state := &nlCoreTracker{manager: m, extraction: extraction, context: make(map[ast.ASTHandle]struct{}), usedSet: make(map[ast.ASTHandle]struct{})}
	for _, a := range extraction.Assumptions {
		state.context[a] = struct{}{}
	}
	return state, nil
}

// learn partitions the solver core with the source's swap-and-pop ordering,
// accumulates context assumptions, and adds a blocking clause when there are
// nonlinear interface assumptions. It returns false for a final context core.
func (s *nlCoreTracker) learn(core []ast.ASTHandle, nonlinear *tactic.Goal) (_ bool, resultErr error) {
	if s == nil || s.manager == nil || nonlinear == nil || nonlinear.Manager() != s.manager {
		return false, ast.ErrASTDomain
	}
	if err := nonlinear.CheckOwner(); err != nil {
		return false, err
	}
	external := append([]ast.ASTHandle(nil), core...)
	for i := 0; i < len(external); {
		item := external[i]
		if _, ok := s.context[item]; !ok {
			i++
			continue
		}
		if _, ok := s.usedSet[item]; !ok {
			s.usedSet[item] = struct{}{}
			s.used = append(s.used, item)
		}
		external[i] = external[len(external)-1]
		external = external[:len(external)-1]
	}
	if len(external) == 0 {
		return false, nil
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, s.manager.DelRef(&refs[i]))
		}
	}()
	literals := make([]ast.ASTHandle, len(external))
	for i, atom := range external {
		literal, err := s.manager.MkNot(atom)
		if err != nil {
			return false, err
		}
		ref, err := s.manager.NewRef(literal)
		if err != nil {
			return false, err
		}
		refs = append(refs, ref)
		literals[i] = literal
	}
	clause, err := s.manager.MkOrArray(literals)
	if err != nil {
		return false, err
	}
	if err = nonlinear.AssertWithProof(clause, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return false, err
	}
	return true, nil
}

// finish replaces the input goal by false and joins all context dependencies
// used by the solver over the preceding iterations.
func (s *nlCoreTracker) finish(g *tactic.Goal, core *tactic.DependencyRef) (resultErr error) {
	if s == nil || s.manager == nil || g == nil || g.Manager() != s.manager || core == nil {
		return ast.ErrASTDomain
	}
	if err := g.Reset(); err != nil {
		return err
	}
	var joined tactic.DependencyRef
	defer func() { resultErr = errors.Join(resultErr, joined.Close()) }()
	for _, assumption := range s.used {
		dependency, ok := s.extraction.BoolToDependency[assumption]
		if !ok {
			return ast.ErrASTDomain
		}
		leaf, err := s.manager.DependencyLeaf(dependency)
		if err != nil {
			return err
		}
		var ownedLeaf tactic.DependencyRef
		if err = ownedLeaf.Set(s.manager, leaf); err != nil {
			return err
		}
		value, err := s.manager.DependencyJoin(joined.Get(), leaf)
		if err == nil {
			err = joined.Set(s.manager, value)
		}
		if closeErr := ownedLeaf.Close(); err == nil {
			err = closeErr
		} else {
			err = errors.Join(err, closeErr)
		}
		if err != nil {
			return err
		}
	}
	if err := core.Set(s.manager, joined.Get()); err != nil {
		return err
	}
	falsehood, err := s.manager.MkFalse()
	if err != nil {
		return err
	}
	return g.AssertWithProof(falsehood, ast.ASTHandle{}, joined.Get())
}
