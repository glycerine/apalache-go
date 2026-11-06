package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp
// lemma_array_eq_generalizer.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (p *PredicateTransformer) collectArrayConstants(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	arrayFamily, err := p.manager.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	seenNodes := make(map[ast.ASTHandle]bool)
	seenDecls := make(map[ast.ASTHandle]bool)
	var selectedSort ast.ASTHandle
	result := make([]ast.ASTHandle, 0, 8)
	type frame struct {
		node  ast.ASTHandle
		ready bool
	}
	todo := []frame{{node: root}}
	for len(todo) != 0 {
		last := len(todo) - 1
		currentFrame := todo[last]
		current := currentFrame.node
		todo = todo[:last]
		if !currentFrame.ready {
			if seenNodes[current] {
				continue
			}
			seenNodes[current] = true
			kind, err := p.manager.GetKind(current)
			if err != nil {
				return nil, err
			}
			if kind == ast.ASTQuantifier {
				var children []ast.ASTHandle
				body, err := p.manager.GetQuantifierBody(current)
				if err != nil {
					return nil, err
				}
				children = append(children, body)
				patterns, err := p.manager.GetNumPatterns(current)
				if err != nil {
					return nil, err
				}
				for j := uint32(0); j < patterns; j++ {
					child, err := p.manager.GetPattern(current, j)
					if err != nil {
						return nil, err
					}
					children = append(children, child)
				}
				noPatterns, err := p.manager.GetNumNoPatterns(current)
				if err != nil {
					return nil, err
				}
				for j := uint32(0); j < noPatterns; j++ {
					child, err := p.manager.GetNoPattern(current, j)
					if err != nil {
						return nil, err
					}
					children = append(children, child)
				}
				for j := len(children); j > 0; j-- {
					todo = append(todo, frame{node: children[j-1]})
				}
				continue
			}
			if kind != ast.ASTApp {
				continue
			}
			todo = append(todo, frame{node: current, ready: true})
			count, err := p.manager.GetNumArgs(current)
			if err != nil {
				return nil, err
			}
			for j := count; j > 0; j-- {
				child, err := p.manager.GetArg(current, j-1)
				if err != nil {
					return nil, err
				}
				todo = append(todo, frame{node: child})
			}
			continue
		}
		sort, err := p.manager.GetSort(current)
		if err != nil {
			return nil, err
		}
		sortFamily, err := p.manager.GetFamilyID(sort)
		if err != nil {
			return nil, err
		}
		decl, err := p.manager.GetDecl(current)
		if err != nil {
			return nil, err
		}
		declFamily, err := p.manager.GetFamilyID(decl)
		if err != nil {
			return nil, err
		}
		if sortFamily == arrayFamily && declFamily == ast.NullFamilyID && !seenDecls[decl] {
			// Source collect_array_proc compares only arrays of the first
			// encountered sort, so candidate equalities stay well-sorted.
			if selectedSort.IsNull() || sort == selectedSort {
				selectedSort = sort
				seenDecls[decl] = true
				result = append(result, decl)
			}
		}

	}
	return result, nil
}

func (p *PredicateTransformer) strengthenArrayEqualities(core []ast.ASTHandle) (result []ast.ASTRef, applicable bool, resultErr error) {
	conjunction, err := p.manager.MkAndArray(core)
	if err != nil {
		return nil, false, err
	}
	declarations, err := p.collectArrayConstants(conjunction)
	if err != nil || len(declarations) <= 1 || len(declarations) >= 8 {
		if err != nil {
			return nil, false, err
		}
		refs, err := refsFromHandles(p.manager, core)
		return refs, false, err
	}
	// Source core, eqs and lits are owning expr_ref_vectors. Keep candidates
	// alive across speculative solver pops and transfer owning output refs.
	coreRefs, err := refsFromHandles(p.manager, core)
	if err != nil {
		return nil, false, err
	}
	attempt := &arrayGeneralizerAttempt{manager: p.manager, core: coreRefs}
	defer func() {
		resultErr = finishArrayGeneralizerAttempt(attempt, resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	equalities := make([]ast.ASTHandle, 0, len(declarations)*(len(declarations)-1)/2)
	for left := 0; left < len(declarations); left++ {
		leftValue, err := p.manager.MkApp(declarations[left], nil)
		if err != nil {
			return nil, false, err
		}
		for right := left + 1; right < len(declarations); right++ {
			rightValue, err := p.manager.MkApp(declarations[right], nil)
			if err != nil {
				return nil, false, err
			}
			equality, err := p.manager.MkEq(leftValue, rightValue)
			if err != nil {
				return nil, false, err
			}
			ref, err := p.manager.NewRef(equality)
			if err != nil {
				return nil, false, err
			}
			attempt.candidates = append(attempt.candidates, ref)
			equalities = append(equalities, equality)
		}
	}
	// Source constructs this kernel from manager::fparams2(), the second
	// solver family (Go pool index 1), not the third context pool.
	checker, err := p.owner.Fresh(1)
	if err != nil {
		return nil, false, err
	}
	attempt.checker = checker
	result = make([]ast.ASTRef, 0, len(core))
	for _, literal := range core {
		if err = checker.Push(); err != nil {
			return nil, false, err
		}
		if err = checker.Assert(literal); err != nil {
			return nil, false, errors.Join(fmt.Errorf("assert array core literal: %w", err), checker.Pop(1))
		}
		strengthened := literal
		for _, equality := range equalities {
			if err = checker.Push(); err != nil {
				return nil, false, errors.Join(err, checker.Pop(1))
			}
			if err = checker.Assert(equality); err != nil {
				return nil, false, errors.Join(fmt.Errorf("assert candidate array equality: %w", err), checker.Pop(2))
			}
			status, checkErr := checker.CheckSat(nil)
			popErr := checker.Pop(1)
			if checkErr != nil {
				return nil, false, errors.Join(fmt.Errorf("check candidate array equality: %w", checkErr), popErr, checker.Pop(1))
			}
			if popErr != nil {
				return nil, false, errors.Join(popErr, checker.Pop(1))
			}
			if status == lbool.False {
				strengthened, err = p.manager.MkNot(equality)
				if err != nil {
					return nil, false, errors.Join(err, checker.Pop(1))
				}
				break
			}
		}
		ref, err := p.manager.NewRef(strengthened)
		if err != nil {
			return nil, false, errors.Join(err, checker.Pop(1))
		}
		result = append(result, ref)
		attempt.output = result
		if err = checker.Pop(1); err != nil {
			return nil, false, err
		}
	}
	return result, true, nil
}

func (p *PredicateTransformer) generalizeArrayEqualityLemma(obligation *ProofObligation, lemma *Lemma) (_ *Lemma, resultErr error) {
	cube, err := lemma.Cube()
	if err != nil {
		return lemma, err
	}
	strengthened, applicable, err := p.strengthenArrayEqualities(cube)
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, strengthened)) }()
	if err != nil {
		return lemma, err
	}
	if !applicable {
		return lemma, nil
	}
	level, err := lemma.Level()
	if err != nil {
		return lemma, err
	}
	handles := make([]ast.ASTHandle, len(strengthened))
	for i, ref := range strengthened {
		handles[i], err = p.manager.RefGet(ref)
		if err != nil {
			return lemma, err
		}
	}
	core, usesLevel, inductive, err := p.checkInductiveCube(handles, level)
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, core)) }()
	if err != nil || !inductive {
		return lemma, err
	}
	handles = make([]ast.ASTHandle, len(core))
	for i, ref := range core {
		handles[i], err = p.manager.RefGet(ref)
		if err != nil {
			return lemma, err
		}
	}
	return NewLemmaFromProofObligation(obligation, handles, usesLevel)
}
