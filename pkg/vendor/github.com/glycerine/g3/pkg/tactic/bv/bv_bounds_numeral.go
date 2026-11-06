package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp numeral decoding.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func bvBoundsNumeral(m *ast.ASTManager, h ast.ASTHandle) (*big.Int, uint32, bool, error) {
	value, err := m.NumeralService().NewValue()
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { _ = m.NumeralService().Del(&value) }()
	var width uint32
	yes, err := m.BitVectorNumeralValue(h, &value, &width)
	if err != nil || !yes {
		return nil, 0, yes, err
	}
	text, err := m.NumeralService().String(&value)
	if err != nil {
		return nil, 0, false, err
	}
	n, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, 0, false, ast.ErrASTDomain
	}
	return n, width, true, nil
}
func bvBoundsMakeNumeral(m *ast.ASTManager, value *big.Int, width uint32) (h ast.ASTHandle, err error) {
	n, err := m.NumeralService().NewValue()
	if err != nil {
		return h, err
	}
	if err = m.NumeralService().SetString(&n, value.String()); err != nil {
		return h, err
	}
	defer func() { err = errors.Join(err, m.NumeralService().Del(&n)) }()
	return m.BitVectorNumeral(&n, width)
}
