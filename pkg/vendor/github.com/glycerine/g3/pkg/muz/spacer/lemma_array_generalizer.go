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
	result := make([]ast.ASTHandle, 0, 8)
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if seenNodes[current] {
			continue
		}
		seenNodes[current] = true
		kind, err := p.manager.GetKind(current)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTQuantifier || kind == ast.ASTVar {
			continue
		}
		if kind != ast.ASTApp {
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
			seenDecls[decl] = true
			result = append(result, decl)
		}
		count, err := p.manager.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		for index := count; index > 0; index-- {
			argument, err := p.manager.GetArg(current, index-1)
			if err != nil {
				return nil, err
			}
			todo = append(todo, argument)
		}
	}
	return result, nil
}

func (p *PredicateTransformer) strengthenArrayEqualities(core []ast.ASTHandle) (_ []ast.ASTHandle, applicable bool, resultErr error) {
	conjunction, err := p.manager.MkAndArray(core)
	if err != nil {
		return nil, false, err
	}
	declarations, err := p.collectArrayConstants(conjunction)
	if err != nil || len(declarations) <= 1 || len(declarations) >= 8 {
		return append([]ast.ASTHandle(nil), core...), false, err
	}
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
			equalities = append(equalities, equality)
		}
	}
	checker, err := p.owner.Fresh(2)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, checker.Close()) }()
	result := make([]ast.ASTHandle, 0, len(core))
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
		if err = checker.Pop(1); err != nil {
			return nil, false, err
		}
		result = append(result, strengthened)
	}
	return result, true, nil
}

func (p *PredicateTransformer) generalizeArrayEqualityLemma(obligation *ProofObligation, lemma *Lemma) (*Lemma, error) {
	cube, err := lemma.Cube()
	if err != nil {
		return lemma, err
	}
	strengthened, applicable, err := p.strengthenArrayEqualities(cube)
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
	core, usesLevel, inductive, err := p.checkInductiveCube(strengthened, level)
	if err != nil || !inductive {
		return lemma, err
	}
	return NewLemmaFromProofObligation(obligation, core, usesLevel)
}
