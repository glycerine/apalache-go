package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp:318-439.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *Converter) evaluatedConstant(mc *model.ModelCore, expression ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	declaration, err := c.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	value, err := mc.ConstantInterpretation(declaration)
	return value, declaration, err
}

func (c *Converter) bvUint64(value ast.ASTHandle) (number uint64, found bool, result error) {
	service := c.manager.NumeralService()
	numeral, err := service.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { result = errors.Join(result, service.Del(&numeral)) }()
	var width uint32
	found, err = c.manager.BitVectorNumeralValue(value, &numeral, &width)
	if err != nil || !found {
		return 0, found, err
	}
	number, err = service.Uint64(&numeral)
	return number, true, err
}

func (c *Converter) convertConstants(source, target *model.ModelCore, seen map[ast.ASTHandle]struct{}) error {
	for declaration, encoded := range c.constants {
		sort, err := c.manager.GetRange(declaration)
		if err != nil {
			return err
		}
		sign, err := c.manager.GetArg(encoded, 0)
		if err != nil {
			return err
		}
		signDecl, err := c.manager.GetDecl(sign)
		if err != nil {
			return err
		}
		arity, err := c.manager.GetArity(signDecl)
		if err != nil {
			return err
		}
		var converted ast.ASTHandle
		if arity != 0 {
			packed, e := c.manager.GetArg(sign, 0)
			if e != nil {
				return e
			}
			packedValue, packedDecl, e := c.evaluatedConstant(source, packed)
			if e != nil {
				return e
			}
			seen[packedDecl] = struct{}{}
			if packedValue.IsNull() {
				ebits, sbits, _, e := c.manager.FloatingPointSortShape(sort)
				if e != nil {
					return e
				}
				packedValue, e = c.manager.BitVectorNumeralUnsigned(0, ebits+sbits)
				if e != nil {
					return e
				}
			}
			converted, err = c.ConvertPackedBV(nil, sort, packedValue)
		} else {
			fields := make([]ast.ASTHandle, 3)
			for i := range fields {
				field, e := c.manager.GetArg(encoded, uint32(i))
				if e != nil {
					return e
				}
				fields[i], signDecl, e = c.evaluatedConstant(source, field)
				if e != nil {
					return e
				}
				seen[signDecl] = struct{}{}
			}
			ebits, sbits, _, e := c.manager.FloatingPointSortShape(sort)
			if e != nil {
				return e
			}
			for i, width := range []uint32{1, ebits, sbits - 1} {
				if fields[i].IsNull() {
					fields[i], e = c.manager.BitVectorNumeralUnsigned(0, width)
					if e != nil {
						return e
					}
				}
			}
			converted, err = c.ConvertBV2FP(sort, fields[0], fields[1], fields[2])
		}
		if err != nil {
			return err
		}
		if err = target.RegisterConstant(declaration, converted); err != nil {
			return err
		}
	}
	return nil
}

func (c *Converter) convertRoundingConstants(source, target *model.ModelCore, seen map[ast.ASTHandle]struct{}) error {
	for declaration, wrapped := range c.roundingConstants {
		bits, err := c.manager.GetArg(wrapped, 0)
		if err != nil {
			return err
		}
		value, bitsDecl, err := c.evaluatedConstant(source, bits)
		if err != nil {
			return err
		}
		seen[bitsDecl] = struct{}{}
		if value.IsNull() {
			value, err = c.manager.BitVectorNumeralUnsigned(4, 3)
			if err != nil {
				return err
			}
		}
		converted, err := c.ConvertBV2RM(value)
		if err != nil {
			return err
		}
		if err = target.RegisterConstant(declaration, converted); err != nil {
			return err
		}
	}
	return nil
}

func (c *Converter) convertMinMaxSpecials(source, target *model.ModelCore, seen map[ast.ASTHandle]struct{}) error {
	for declaration, signs := range c.minMaxSigns {
		sort, err := c.manager.GetRange(declaration)
		if err != nil {
			return err
		}
		positiveZero, err := c.manager.MkFPAZeroOfSort(sort, false)
		if err != nil {
			return err
		}
		negativeZero, err := c.manager.MkFPAZeroOfSort(sort, true)
		if err != nil {
			return err
		}
		codes := [2]uint64{}
		for i, sign := range signs {
			value, signDecl, e := c.evaluatedConstant(source, sign)
			if e != nil {
				return e
			}
			seen[signDecl] = struct{}{}
			if !value.IsNull() {
				codes[i], _, e = c.bvUint64(value)
				if e != nil {
					return e
				}
			}
		}
		arity, err := c.manager.GetArity(declaration)
		if err != nil {
			return err
		}
		interpretation, err := model.NewFunctionInterpretation(c.manager, arity)
		if err != nil {
			return err
		}
		registered := false
		defer func() {
			if !registered {
				_ = interpretation.Close()
			}
		}()
		if codes[0] != codes[1] {
			result := positiveZero
			if codes[0] == 1 {
				result = negativeZero
			}
			if err = interpretation.InsertNewEntry([]ast.ASTHandle{positiveZero, negativeZero}, result); err != nil {
				return err
			}
		}
		fallback := positiveZero
		if codes[1] == 1 {
			fallback = negativeZero
		}
		if err = interpretation.SetElse(fallback); err != nil {
			return err
		}
		if err = target.RegisterFunction(declaration, interpretation); err != nil {
			return err
		}
		registered = true
	}
	return nil
}

// ConvertModel reconstructs the source converter's constants, rounding-mode
// constants, and min/max unspecified interpretations. It returns the converted
// BV declarations consumed from the source model.
func (c *Converter) ConvertModel(source, target *model.ModelCore) (map[ast.ASTHandle]struct{}, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if source == nil || target == nil {
		return nil, ast.ErrASTHandle
	}
	seen := make(map[ast.ASTHandle]struct{})
	if err := c.convertConstants(source, target, seen); err != nil {
		return nil, err
	}
	if err := c.convertRoundingConstants(source, target, seen); err != nil {
		return nil, err
	}
	if err := c.convertMinMaxSpecials(source, target, seen); err != nil {
		return nil, err
	}
	if err := c.convertUFs(source, target, seen); err != nil {
		return nil, err
	}
	return seen, nil
}

// ConvertFunctions reconstructs converted uninterpreted functions and
// min/max auxiliary interpretations while leaving source constants alone.
// SMT model generation already assigns wrapper-backed FP/RM constants from
// their theory value procedures, so replacing them through the standalone
// constant-cache path would discard those assignments.
func (c *Converter) ConvertFunctions(source, target *model.ModelCore) (map[ast.ASTHandle]struct{}, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if source == nil || target == nil {
		return nil, ast.ErrASTHandle
	}
	seen := make(map[ast.ASTHandle]struct{})
	if err := c.convertMinMaxSpecials(source, target, seen); err != nil {
		return nil, err
	}
	if err := c.convertUFs(source, target, seen); err != nil {
		return nil, err
	}
	return seen, nil
}
