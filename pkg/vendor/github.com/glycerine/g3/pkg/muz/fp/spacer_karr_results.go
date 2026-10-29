package fp

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp
// add_invariant_model_converter.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) spacerKarrFormula(declaration ast.ASTHandle, matrix rel.KarrMatrix) (ast.ASTHandle, error) {
	zero, err := e.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjuncts := make([]ast.ASTHandle, 0, matrix.Len())
	for i := range matrix.A {
		terms := make([]ast.ASTHandle, 0, len(matrix.A[i])+1)
		for column := range matrix.A[i] {
			coefficient := &matrix.A[i][column]
			if coefficient.Sign() == 0 {
				continue
			}
			sort, getErr := e.manager.GetDomain(declaration, uint32(column))
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			term, getErr := e.manager.MkVar(uint32(column), sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
				numeral, numeralErr := e.manager.ArithmeticNumeralRat(coefficient, true)
				if numeralErr != nil {
					return ast.ASTHandle{}, numeralErr
				}
				term, numeralErr = e.manager.MkArithmeticMul(numeral, term)
				if numeralErr != nil {
					return ast.ASTHandle{}, numeralErr
				}
			}
			terms = append(terms, term)
		}
		if matrix.B[i].Sign() != 0 {
			constant, constantErr := e.manager.ArithmeticNumeralRat(&matrix.B[i], true)
			if constantErr != nil {
				return ast.ASTHandle{}, constantErr
			}
			terms = append(terms, constant)
		}
		lhs := zero
		if len(terms) != 0 {
			lhs = terms[0]
			for j := 1; j < len(terms); j++ {
				lhs, err = e.manager.MkArithmeticAdd(lhs, terms[j])
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
		var formula ast.ASTHandle
		if matrix.Eq[i] {
			formula, err = e.manager.MkEq(lhs, zero)
		} else {
			formula, err = e.manager.MkArithmeticGE(lhs, zero)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		conjuncts = append(conjuncts, formula)
	}
	if len(conjuncts) == 1 {
		return conjuncts[0], nil
	}
	return e.manager.MkAndArray(conjuncts)
}

func (e *PropositionalEngine) applySpacerKarrInvariant(destination *model.ModelCore, declaration ast.ASTHandle, matrix rel.KarrMatrix) (resultErr error) {
	invariant, err := e.spacerKarrFormula(declaration, matrix)
	if err != nil {
		return err
	}
	interpretation, err := destination.FunctionInterpretation(declaration)
	if err != nil {
		return err
	}
	if interpretation == nil {
		arity, arityErr := e.manager.GetArity(declaration)
		if arityErr != nil || arity == 0 {
			return errors.Join(arityErr, ast.ErrASTDomain)
		}
		interpretation, err = model.NewFunctionInterpretation(e.manager, arity)
		if err != nil {
			return err
		}
		transferred := false
		defer func() {
			if !transferred {
				resultErr = errors.Join(resultErr, interpretation.Close())
			}
		}()
		body, makeErr := e.manager.MkFalse()
		if makeErr == nil {
			makeErr = interpretation.SetElse(body)
		}
		if makeErr == nil {
			makeErr = destination.RegisterFunction(declaration, interpretation)
		}
		if makeErr != nil {
			return makeErr
		}
		transferred = true
		return nil
	}
	if interpretation.NumEntries() != 0 {
		return fmt.Errorf("muz/fp: Karr predicate interpretation contains entries")
	}
	partial, err := interpretation.IsPartial()
	if err != nil || partial {
		return err
	}
	body, err := interpretation.Else()
	if err != nil {
		return err
	}
	body, err = e.manager.MkAnd(body, invariant)
	if err != nil {
		return err
	}
	return interpretation.SetElse(body)
}

func (e *PropositionalEngine) applySpacerKarrInvariants(destination *model.ModelCore) error {
	for declaration, matrix := range e.spacerKarr {
		if matrix.Len() != 0 {
			if err := e.applySpacerKarrInvariant(destination, declaration, matrix); err != nil {
				return err
			}
		}
	}
	return nil
}
