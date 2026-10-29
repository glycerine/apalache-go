package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/model"
)

// Converter rebuilds floating-point interpretations from the bit-vector model
// produced by ast/fpa. It owns retained copies of the source converter maps.
type Converter struct {
	self              *Converter
	manager           *ast.ASTManager
	constants         map[ast.ASTHandle]ast.ASTHandle
	roundingConstants map[ast.ASTHandle]ast.ASTHandle
	convertedUFs      map[ast.ASTHandle]ast.ASTHandle
	minMaxSigns       map[ast.ASTHandle][2]ast.ASTHandle
	header            uint64
	closed            bool
}

func NewConverter(m *ast.ASTManager, source *astfpa.Converter) (owner *Converter, result error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	bytes := uint64(reflect.TypeOf(Converter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	owner = &Converter{
		manager: m, header: bytes,
		constants: make(map[ast.ASTHandle]ast.ASTHandle), roundingConstants: make(map[ast.ASTHandle]ast.ASTHandle),
		convertedUFs: make(map[ast.ASTHandle]ast.ASTHandle), minMaxSigns: make(map[ast.ASTHandle][2]ast.ASTHandle),
	}
	owner.self = owner
	if source == nil {
		return owner, nil
	}
	snapshot, err := source.Snapshot()
	if err != nil {
		return owner, errors.Join(err, owner.Close())
	}
	retain := func(key, value ast.ASTHandle) error {
		if err := m.IncRef(key); err != nil {
			return err
		}
		if err := m.IncRef(value); err != nil {
			return errors.Join(err, m.DecRef(key))
		}
		return nil
	}
	for key, value := range snapshot.Constants {
		if err = retain(key, value); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		owner.constants[key] = value
	}
	for key, value := range snapshot.RoundingConstants {
		if err = retain(key, value); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		owner.roundingConstants[key] = value
	}
	for key, value := range snapshot.ConvertedUFs {
		if err = retain(key, value); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		owner.convertedUFs[key] = value
	}
	for key, values := range snapshot.MinMaxSigns {
		if err = m.IncRef(key); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		if err = m.IncRef(values[0]); err != nil {
			return owner, errors.Join(err, m.DecRef(key), owner.Close())
		}
		if err = m.IncRef(values[1]); err != nil {
			return owner, errors.Join(err, m.DecRef(values[0]), m.DecRef(key), owner.Close())
		}
		owner.minMaxSigns[key] = values
	}
	return owner, nil
}

func (c *Converter) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := c.manager.ProofsEnabled()
	return err
}

func (c *Converter) Close() (result error) {
	if c == nil || c.self != c || c.closed {
		return nil
	}
	for key, value := range c.constants {
		result = errors.Join(result, c.manager.DecRef(value), c.manager.DecRef(key))
		delete(c.constants, key)
	}
	for key, value := range c.roundingConstants {
		result = errors.Join(result, c.manager.DecRef(value), c.manager.DecRef(key))
		delete(c.roundingConstants, key)
	}
	for key, value := range c.convertedUFs {
		result = errors.Join(result, c.manager.DecRef(value), c.manager.DecRef(key))
		delete(c.convertedUFs, key)
	}
	for key, values := range c.minMaxSigns {
		result = errors.Join(result, c.manager.DecRef(values[1]), c.manager.DecRef(values[0]), c.manager.DecRef(key))
		delete(c.minMaxSigns, key)
	}
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.closed = true
	return result
}

// ConvertBV2FP rebuilds an FPA numeral from evaluated IEEE component fields.
func (c *Converter) ConvertBV2FP(sort, sign, exponent, significand ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	for _, field := range []struct {
		value ast.ASTHandle
		width uint32
	}{{sign, 1}, {exponent, ebits}, {significand, sbits - 1}} {
		if width, e := c.manager.BitVectorSize(field.value); e != nil || width != field.width {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	family, err := c.manager.RegisterFloatingPointPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	components, err := c.manager.MkFamilyApp(family, ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := c.manager.GetDecl(components)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, found, err := c.manager.RewriteFPAFPComponentsNumerals(declaration, sign, exponent, significand)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// ConvertBV2RM maps the converter's three-bit rounding code back to an FPA
// rounding-mode numeral. Values outside 0..3 use toward-zero.
func (c *Converter) ConvertBV2RM(bits ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	service := c.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	var width uint32
	found, err := c.manager.BitVectorNumeralValue(bits, &value, &width)
	if err != nil || !found || width != 3 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	n, err := service.Uint64(&value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind := ast.FPAOpRMTowardZero
	switch n {
	case 0:
		kind = ast.FPAOpRMNearestEven
	case 1:
		kind = ast.FPAOpRMNearestAway
	case 2:
		kind = ast.FPAOpRMTowardPositive
	case 3:
		kind = ast.FPAOpRMTowardNegative
	}
	family, err := c.manager.RegisterFloatingPointPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkFamilyApp(family, kind, nil, nil)
}

// ConvertPackedBV evaluates a packed BV constant when necessary, splits its
// IEEE fields, and reconstructs an FPA numeral. Missing interpretations use 0.
func (c *Converter) ConvertPackedBV(mc *model.ModelCore, sort, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width := ebits + sbits
	var probeWidth uint32
	service := c.manager.NumeralService()
	probe, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	found, probeErr := c.manager.BitVectorNumeralValue(value, &probe, &probeWidth)
	err = errors.Join(probeErr, service.Del(&probe))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found {
		if mc == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		declaration, e := c.manager.GetDecl(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		value, e = mc.ConstantInterpretation(declaration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if value.IsNull() {
			value, e = c.manager.BitVectorNumeralUnsigned(0, width)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	}
	extract := func(high, low uint32) (node ast.ASTHandle, result error) {
		numerals, e := ast.NewBitVectorExtractNumerals(c.manager)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		defer func() { result = errors.Join(result, numerals.Close()) }()
		var sourceWidth uint32
		matched, e := numerals.Read(value, &sourceWidth)
		if e != nil || !matched || sourceWidth != width {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return ast.ASTHandle{}, e
		}
		node, e = numerals.Evaluate(high, low)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = c.manager.IncRef(node); e != nil {
			return ast.ASTHandle{}, e
		}
		if e = numerals.SourceEnd(); e != nil {
			return ast.ASTHandle{}, errors.Join(e, c.manager.DecRef(node))
		}
		return node, nil
	}
	sign, err := extract(width-1, width-1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer c.manager.DecRef(sign)
	exponent, err := extract(width-2, sbits-1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer c.manager.DecRef(exponent)
	significand, err := extract(sbits-2, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer c.manager.DecRef(significand)
	return c.ConvertBV2FP(sort, sign, exponent, significand)
}
