package api

// Source: src/api/api_fpa.cpp:733-869,1191-1206.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func floatingPointFormatParameters(m *ast.ASTManager, exponentBits, significandBits uint32) ([]ast.ASTParameter, error) {
	first, err := m.NewParameterUint(exponentBits)
	if err != nil {
		return nil, err
	}
	second, err := m.NewParameterUint(significandBits)
	if err != nil {
		_ = m.DelParameter(&first)
		return nil, err
	}
	return []ast.ASTParameter{first, second}, nil
}

func (c *SMTContext) floatingPointToFormat(kind int32, target ast.ASTHandle, arguments ...ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	m, family, err := c.floatingPointManager()
	if err != nil {
		return result, err
	}
	exponentBits, significandBits, matched, err := m.FloatingPointSortShape(target)
	if err != nil {
		return result, err
	}
	if !matched {
		return result, ast.ErrASTDomain
	}
	parameters, err := floatingPointFormatParameters(m, exponentBits, significandBits)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, m.DelParameter(&parameters[1]), m.DelParameter(&parameters[0]))
	}()
	return m.MkFamilyApp(family, kind, parameters, arguments)
}

func (c *SMTContext) FloatingPointFromIEEEBitVector(value, target ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.requireFloatingPointConversionSource(value, APISortBitVector); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.floatingPointToFormat(ast.FPAOpToFP, target, value)
}
func (c *SMTContext) FloatingPointConvert(rounding, value, target ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.requireFloatingPointConversionSource(value, APISortFloatingPoint); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.floatingPointToFormat(ast.FPAOpToFP, target, rounding, value)
}
func (c *SMTContext) FloatingPointFromReal(rounding, value, target ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.requireFloatingPointConversionSource(value, APISortReal); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.floatingPointToFormat(ast.FPAOpToFP, target, rounding, value)
}
func (c *SMTContext) FloatingPointFromSignedBitVector(rounding, value, target ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.requireFloatingPointConversionSource(value, APISortBitVector); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.floatingPointToFormat(ast.FPAOpToFP, target, rounding, value)
}
func (c *SMTContext) FloatingPointFromUnsignedBitVector(rounding, value, target ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.requireFloatingPointConversionSource(value, APISortBitVector); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.floatingPointToFormat(ast.FPAOpToFPUnsigned, target, rounding, value)
}

func (c *SMTContext) requireFloatingPointConversionSource(value ast.ASTHandle, expected APISortKind) error {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	sort, err := m.GetSort(value)
	if err != nil {
		return err
	}
	kind, err := c.SortKind(sort)
	if err != nil {
		return err
	}
	if kind != expected {
		return ast.ErrASTDomain
	}
	return nil
}

func (c *SMTContext) floatingPointToBitVector(kind int32, rounding, value ast.ASTHandle, bits uint32) (result ast.ASTHandle, resultErr error) {
	if bits == 0 {
		return result, ast.ErrASTDomain
	}
	m, family, err := c.floatingPointManager()
	if err != nil {
		return result, err
	}
	parameter, err := m.NewParameterUint(bits)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	return m.MkFamilyApp(family, kind, []ast.ASTParameter{parameter}, []ast.ASTHandle{rounding, value})
}

func (c *SMTContext) FloatingPointToUnsignedBitVector(rounding, value ast.ASTHandle, bits uint32) (ast.ASTHandle, error) {
	return c.floatingPointToBitVector(ast.FPAOpToUBV, rounding, value, bits)
}
func (c *SMTContext) FloatingPointToSignedBitVector(rounding, value ast.ASTHandle, bits uint32) (ast.ASTHandle, error) {
	return c.floatingPointToBitVector(ast.FPAOpToSBV, rounding, value, bits)
}
func (c *SMTContext) FloatingPointToReal(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpToReal, value)
}
func (c *SMTContext) FloatingPointToIEEEBitVector(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.floatingPointApp(ast.FPAOpToIEEEBV, value)
}
