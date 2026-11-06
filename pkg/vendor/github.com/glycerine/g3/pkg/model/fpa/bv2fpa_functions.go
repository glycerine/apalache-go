package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp:177-316,441-472.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *Converter) convertModelRoundingMode(mc *model.ModelCore, value ast.ASTHandle) (ast.ASTHandle, error) {
	if value.IsNull() {
		bits, err := c.manager.BitVectorNumeralUnsigned(4, 3)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.ConvertBV2RM(bits)
	}
	if _, found, err := c.bvUint64(value); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return c.ConvertBV2RM(value)
	}
	declaration, err := c.manager.GetDecl(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	evaluated, err := mc.ConstantInterpretation(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if evaluated.IsNull() {
		evaluated, err = c.manager.BitVectorNumeralUnsigned(4, 3)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return c.ConvertBV2RM(evaluated)
}

// RebuildFloats reconstructs floating-point and rounding-mode subterms using
// the expected source declaration sort at each application argument.
func (c *Converter) RebuildFloats(mc *model.ModelCore, expectedSort, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	_, _, floating, err := c.manager.FloatingPointSortShape(expectedSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if floating {
		if value.IsNull() {
			return c.manager.MkFPAZeroOfSort(expectedSort, false)
		}
		actualSort, e := c.manager.GetSort(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		_, _, alreadyFloating, e := c.manager.FloatingPointSortShape(actualSort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if alreadyFloating {
			return value, nil
		}
		return c.ConvertPackedBV(mc, expectedSort, value)
	}
	rounding, err := c.manager.IsRoundingModeSort(expectedSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rounding {
		if !value.IsNull() {
			actualSort, e := c.manager.GetSort(value)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			alreadyRounding, e := c.manager.IsRoundingModeSort(actualSort)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if alreadyRounding {
				return value, nil
			}
		}
		return c.convertModelRoundingMode(mc, value)
	}
	if value.IsNull() {
		return ast.ASTHandle{}, nil
	}
	kind, err := c.manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return value, err
	}
	declaration, err := c.manager.GetDecl(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arity, err := c.manager.GetArity(declaration)
	if err != nil || arity == 0 {
		return value, err
	}
	arguments := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		argument, e := c.manager.GetArg(value, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		domain, e := c.manager.GetDomain(declaration, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments[i], e = c.RebuildFloats(mc, domain, argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return c.manager.MkApp(declaration, arguments)
}

func (c *Converter) convertFunctionInterpretation(source *model.ModelCore, declaration, convertedDeclaration ast.ASTHandle) (*model.FunctionInterpretation, error) {
	arity, err := c.manager.GetArity(convertedDeclaration)
	if err != nil {
		return nil, err
	}
	result, err := model.NewFunctionInterpretation(c.manager, arity)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*model.FunctionInterpretation, error) {
		return nil, errors.Join(err, result.Close())
	}
	sourceInterpretation, err := source.FunctionInterpretation(convertedDeclaration)
	if err != nil {
		return fail(err)
	}
	if sourceInterpretation == nil {
		return result, nil
	}
	rangeSort, err := c.manager.GetRange(declaration)
	if err != nil {
		return fail(err)
	}
	_, _, floatingRange, err := c.manager.FloatingPointSortShape(rangeSort)
	if err != nil {
		return fail(err)
	}
	for i := uint32(0); i < sourceInterpretation.NumEntries(); i++ {
		entry, e := sourceInterpretation.Entry(i)
		if e != nil {
			return fail(e)
		}
		arguments := make([]ast.ASTHandle, arity)
		for j := uint32(0); j < arity; j++ {
			argument, e := entry.Argument(j)
			if e != nil {
				return fail(e)
			}
			domain, e := c.manager.GetDomain(declaration, j)
			if e != nil {
				return fail(e)
			}
			arguments[j], e = c.RebuildFloats(source, domain, argument)
			if e != nil {
				return fail(e)
			}
		}
		entryResult, e := entry.Result()
		if e != nil {
			return fail(e)
		}
		convertedResult, e := c.RebuildFloats(source, rangeSort, entryResult)
		if e != nil {
			return fail(e)
		}
		existing, e := result.FindEntry(arguments)
		if e != nil {
			return fail(e)
		}
		if existing == nil {
			if e = result.InsertNewEntry(arguments, convertedResult); e != nil {
				return fail(e)
			}
		} else if floatingRange {
			old, e := existing.Result()
			if e != nil {
				return fail(e)
			}
			equal, e := c.manager.AreEqual(old, convertedResult)
			if e != nil || !equal {
				if e == nil {
					e = ast.ErrASTDomain
				}
				return fail(e)
			}
		}
	}
	fallback, err := sourceInterpretation.Else()
	if err != nil {
		return fail(err)
	}
	if !fallback.IsNull() {
		fallback, err = c.RebuildFloats(source, rangeSort, fallback)
		if err != nil {
			return fail(err)
		}
		if err = result.SetElse(fallback); err != nil {
			return fail(err)
		}
	}
	return result, nil
}

func (c *Converter) convertUFs(source, target *model.ModelCore, seen map[ast.ASTHandle]struct{}) error {
	for declaration, convertedDeclaration := range c.convertedUFs {
		seen[convertedDeclaration] = struct{}{}
		arity, err := c.manager.GetArity(declaration)
		if err != nil {
			return err
		}
		if arity == 0 {
			rangeSort, e := c.manager.GetRange(declaration)
			if e != nil {
				return e
			}
			if _, e = c.manager.ArrayArity(rangeSort); e == nil {
				// Array-as-function reconstruction is ported with its dedicated
				// model slice; retain the converted declaration as seen here.
				continue
			}
			value, e := source.ConstantInterpretation(convertedDeclaration)
			if e != nil {
				return e
			}
			if !value.IsNull() {
				if e = target.RegisterConstant(declaration, value); e != nil {
					return e
				}
			}
			continue
		}
		family, err := c.manager.GetFamilyID(declaration)
		if err != nil {
			return err
		}
		fpaFamily, err := c.manager.ExistingFloatingPointFamilyID()
		if err != nil {
			return err
		}
		if family != fpaFamily {
			continue
		}
		interpretation, err := c.convertFunctionInterpretation(source, declaration, convertedDeclaration)
		if err != nil {
			return err
		}
		if err = target.RegisterFunction(declaration, interpretation); err != nil {
			_ = interpretation.Close()
			return err
		}
	}
	return nil
}
