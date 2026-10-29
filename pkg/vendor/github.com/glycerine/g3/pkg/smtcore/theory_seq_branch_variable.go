package smtcore

// Source: src/smt/theory_seq.cpp branch_variable, find_branch_candidate and
// assume_equality retained equality case splitting for non-fixed words.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The source sequence theory owns a substitution-free th_rewriter. The
// kernel's preprocessor also carries assertion substitutions and cannot be
// borrowed for unconditional equality simplification here.
type smtSequenceRewriterOwner struct {
	context  *uparams.ParamsContext
	catalog  *uparams.GlobalParamsCatalog
	globals  *uparams.GlobalParamsService
	rewriter *rewriter.TheoryRewriter
}

func (r *smtSequenceRewriterOwner) close() (result error) {
	if r.rewriter != nil {
		result = errors.Join(result, r.rewriter.Close())
	}
	if r.globals != nil {
		result = errors.Join(result, r.globals.Close())
	}
	if r.catalog != nil {
		result = errors.Join(result, r.catalog.Close())
	}
	if r.context != nil {
		result = errors.Join(result, r.context.Close())
	}
	return result
}

func (t *SMTSequenceTheory) equalityRewriter() (_ *rewriter.TheoryRewriter, result error) {
	if t.rewrite != nil {
		return t.rewrite.rewriter, nil
	}
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	m := state.manager
	r := &smtSequenceRewriterOwner{}
	defer func() {
		if result != nil {
			result = errors.Join(result, r.close())
		}
	}()
	r.context, err = uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	if err != nil {
		return nil, err
	}
	r.catalog, err = uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	if err != nil {
		return nil, err
	}
	r.globals, err = uparams.NewGlobalParamsService(m.Symbols, r.context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: r.catalog})
	if err != nil {
		return nil, err
	}
	if err = r.globals.InitializeSource(); err != nil {
		return nil, err
	}
	r.rewriter, err = rewriter.NewTheoryRewriter(m, r.context, r.globals, nil)
	if err != nil {
		return nil, err
	}
	t.rewrite = r
	return r.rewriter, nil
}

func (t *SMTSequenceTheory) assumeSequenceEquality(left, right ast.ASTHandle) (value lbool.LBool, result error) {
	state, err := t.State()
	if err != nil {
		return lbool.Undef, err
	}
	if t.exclusions.Contains(left, right) {
		return lbool.False, nil
	}
	equality, err := state.manager.MkEq(left, right)
	if err != nil {
		return lbool.Undef, err
	}
	original, err := state.manager.NewRef(equality)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, state.manager.DelRef(&original)) }()
	ref, err := state.manager.NewRef(equality)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, state.manager.DelRef(&ref)) }()
	rw, err := t.equalityRewriter()
	if err != nil {
		return lbool.Undef, err
	}
	if err = rw.Rewrite(equality, ref); err != nil {
		return lbool.Undef, err
	}
	rewritten, err := state.manager.RefGet(ref)
	if err != nil {
		return lbool.Undef, err
	}
	if state.manager.IsTrue(rewritten) {
		return lbool.True, nil
	}
	if state.manager.IsFalse(rewritten) {
		return lbool.False, nil
	}
	if err = state.Internalize(left, false); err != nil {
		return lbool.Undef, err
	}
	if err = state.Internalize(right, false); err != nil {
		return lbool.Undef, err
	}
	first, second := state.context.enodes[left], state.context.enodes[right]
	if first == nil || second == nil {
		return lbool.Undef, ast.ErrASTDomain
	}
	if first.Root() == second.Root() {
		return lbool.True, nil
	}
	if state.context.isDisequal(first, second) {
		return lbool.False, nil
	}
	if err = state.markRelevant(left); err != nil {
		return lbool.Undef, err
	}
	if err = state.markRelevant(right); err != nil {
		return lbool.Undef, err
	}
	// Sequence inherits the source base mk_eq_atom AST-ID order.
	firstID, err := first.OwnerID()
	if err != nil {
		return lbool.Undef, err
	}
	secondID, err := second.OwnerID()
	if err != nil {
		return lbool.Undef, err
	}
	if firstID > secondID {
		first, second = second, first
	}
	accepted, err := state.assumeEquality(first, second)
	if err != nil {
		return lbool.Undef, err
	}
	if !accepted {
		return lbool.False, nil
	}
	equality, err = state.manager.MkEq(first.Owner(), second.Owner())
	if err != nil {
		return lbool.Undef, err
	}
	literal, err := state.Literal(equality)
	if err != nil {
		return lbool.Undef, err
	}
	return state.Assignment(literal)
}

func (t *SMTSequenceTheory) sequenceWordsCanBeEqual(left, right []ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	index := 0
	for index < len(left) && index < len(right) {
		distinct, distinctErr := state.manager.AreDistinct(left[index], right[index])
		if distinctErr != nil {
			return false, distinctErr
		}
		if distinct {
			return false, nil
		}
		equal, equalErr := state.manager.AreEqual(left[index], right[index])
		if equalErr != nil {
			return false, equalErr
		}
		if !equal {
			break
		}
		index++
	}
	if index == len(right) {
		left, right = right, left
	}
	if index == len(left) && index < len(right) {
		for _, part := range right[index:] {
			if _, unit, unitErr := state.manager.SequenceUnitArgument(part); unitErr != nil {
				return false, unitErr
			} else if unit {
				return false, nil
			}
		}
	}
	return true, nil
}

func (t *SMTSequenceTheory) branchVariablePrefixEquality(id uint32, left, right []ast.ASTHandle, dependency *smtSequenceDependency) (bool, error) {
	for pass := uint32(0); pass < 2; pass++ {
		key := 2*id + pass
		start := t.branchStart[key]
		found, err := t.findSequenceBranchCandidate(&start, left, right, dependency)
		if err != nil {
			return false, err
		}
		if start != t.branchStart[key] {
			state, err := t.State()
			if err != nil {
				return false, err
			}
			// The source retains this search state even if the chosen equality
			// already holds. Advancing it is real work for the kernel progress gate.
			state.theoryProgress++
		}
		t.branchStart[key] = start
		t.branchLog = append(t.branchLog, key)
		if found {
			return true, nil
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) findSequenceBranchCandidate(start *int, left, right []ast.ASTHandle, dependency *smtSequenceDependency) (progress bool, result error) {
	if len(left) == 0 {
		return false, nil
	}
	variable, err := t.sequenceVariable(left[0])
	if err != nil || !variable {
		return false, err
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	var candidates []ast.ASTRef
	defer func() {
		for i := len(candidates) - 1; i >= 0; i-- {
			result = errors.Join(result, state.manager.DelRef(&candidates[i]))
		}
	}()
	sort, err := state.manager.GetSort(left[0])
	if err != nil {
		return false, err
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return false, err
	}
	emptyRef, err := state.manager.NewRef(empty)
	if err != nil {
		return false, err
	}
	candidates = append(candidates, emptyRef)
	possible, err := t.sequenceWordsCanBeEqual(left[1:], right)
	if err != nil {
		return false, err
	}
	if possible {
		assignment, err := t.assumeSequenceEquality(left[0], empty)
		if err != nil {
			return false, err
		}
		if assignment != lbool.False {
			return true, nil
		}
	}
	for ; *start < len(right); *start++ {
		index := *start
		if left[0] == right[index] {
			return false, nil
		}
		possible, err := t.sequenceWordsCanBeEqual(left[1:], right[index+1:])
		if err != nil {
			return false, err
		}
		if !possible {
			continue
		}
		prefix, err := t.makeWord(right[:index+1], sort)
		if err != nil {
			return false, err
		}
		ref, err := state.manager.NewRef(prefix)
		if err != nil {
			return false, err
		}
		candidates = append(candidates, ref)
		assignment, err := t.assumeSequenceEquality(left[0], prefix)
		if err != nil {
			return false, err
		}
		if assignment != lbool.False {
			*start++
			return true, nil
		}
	}
	for _, part := range right {
		_, unit, err := state.manager.SequenceUnitArgument(part)
		if err != nil || !unit {
			return false, err
		}
	}
	// Reconstruct every feasible prefix, including those before the saved
	// cursor, so an exhaustion conflict retains all of its negative decisions.
	choices := []ast.ASTHandle{empty}
	for index := range right {
		possible, err := t.sequenceWordsCanBeEqual(left[1:], right[index+1:])
		if err != nil {
			return false, err
		}
		if !possible {
			continue
		}
		prefix, err := t.makeWord(right[:index+1], sort)
		if err != nil {
			return false, err
		}
		ref, err := state.manager.NewRef(prefix)
		if err != nil {
			return false, err
		}
		candidates = append(candidates, ref)
		choices = append(choices, prefix)
	}
	conflictDependency := dependency
	for _, candidate := range choices {
		first, second := left[0], candidate
		firstID, err := state.manager.GetASTID(first)
		if err != nil {
			return false, err
		}
		secondID, err := state.manager.GetASTID(second)
		if err != nil {
			return false, err
		}
		if firstID > secondID {
			first, second = second, first
		}
		equality, err := state.manager.MkEq(first, second)
		if err != nil {
			return false, err
		}
		if err = state.Internalize(equality, false); err != nil {
			return false, err
		}
		literal, err := state.Literal(equality)
		if err != nil {
			return false, err
		}
		assignment, err := state.Assignment(literal)
		if err != nil {
			return false, err
		}
		if assignment != lbool.False {
			if assignment == lbool.Undef {
				data, err := state.BooleanData(literal.Var())
				if err != nil {
					return false, err
				}
				if !data.PhaseAvailable || !data.Phase {
					state.theoryProgress++
				}
				data.PhaseAvailable, data.Phase = true, true
			}
			*start = 0
			return true, nil
		}
		conflictDependency = t.dependencies.Join(conflictDependency, t.dependencies.Leaf(smtSequenceAssumption{literal: literal.Negated()}))
	}
	return true, t.setDependencyConflict(conflictDependency)
}
