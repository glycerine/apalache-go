package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_relation::to_formula(tbv/doc).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *TypedUDocRelation) cubeFormula(cube *TBV, owner *typedUDocFormulaOwner) (ast.ASTHandle, error) {
	m := r.factory.manager
	conjunction := make([]ast.ASTHandle, 0)
	for column, sort := range r.sorts {
		low, limit := r.relation.offsets[column], r.relation.offsets[column+1]
		for bit := low; bit < limit; {
			value, err := r.relation.manager.vectors.Get(cube, bit)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if value == TBitX {
				bit++
				continue
			}
			start := bit
			number := new(big.Int)
			for bit < limit {
				value, err = r.relation.manager.vectors.Get(cube, bit)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if value == TBitX {
					break
				}
				if value == TBit1 {
					number.SetBit(number, int(bit-start), 1)
				}
				bit++
			}
			variable, err := owner.hold(m.MkVar(uint32(column), sort))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			bitVector, err := m.IsBitVectorSort(sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			var term, numeral ast.ASTHandle
			if bitVector {
				width := bit - start
				numeral, err = owner.bitVectorNumeral(number, width)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				term = variable
				if start != low || bit != limit {
					term, err = owner.hold(m.MkBitVectorExtract(bit-1-low, start-low, variable))
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
			} else {
				// Source only supports whole-column numerals for finite-domain
				// sorts; partial finite bit-ranges are a BV-only operation.
				if start != low || bit != limit || !number.IsUint64() {
					return ast.ASTHandle{}, ErrDOC
				}
				numeral, err = owner.hold(m.MkDatalogNumeral(number.Uint64(), sort))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				term = variable
			}
			equal, err := owner.hold(m.MkEq(term, numeral))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			conjunction = append(conjunction, equal)
		}
	}
	return owner.hold(m.MkAndArray(conjunction))
}

func (r *TypedUDocRelation) documentFormula(document *Doc, owner *typedUDocFormulaOwner) (ast.ASTHandle, error) {
	positive, err := r.cubeFormula(document.positive, owner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction := []ast.ASTHandle{positive}
	for _, negative := range document.negative.values {
		formula, err := r.cubeFormula(negative, owner)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		formula, err = owner.hold(r.factory.manager.MkNotSimplified(formula))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		conjunction = append(conjunction, formula)
	}
	return owner.hold(r.factory.manager.MkAndArray(conjunction))
}

// ToFormula returns an owning formula over original column-sort variables.
func (r *TypedUDocRelation) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if r.checker != nil {
		formula, err := r.factory.manager.RefGet(r.checker.formula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		return r.factory.manager.NewRef(formula)
	}
	return r.toUncheckedFormula()
}

func (r *TypedUDocRelation) toUncheckedFormula() (result ast.ASTRef, resultErr error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	owner := &typedUDocFormulaOwner{manager: r.factory.manager}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr != nil {
			result = ast.ASTRef{}
		} else {
			result, owner.result = owner.result, ast.ASTRef{}
		}
	}()
	disjunction := make([]ast.ASTHandle, 0, len(r.relation.elems.values))
	for _, document := range r.relation.elems.values {
		formula, err := r.documentFormula(document, owner)
		if err != nil {
			return ast.ASTRef{}, err
		}
		disjunction = append(disjunction, formula)
	}
	formula, err := owner.hold(r.factory.manager.MkOrArray(disjunction))
	if err != nil {
		return ast.ASTRef{}, err
	}
	owner.result, resultErr = r.factory.manager.NewRef(formula)
	return owner.result, resultErr
}
