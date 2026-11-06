package tactic

// Source: src/tactic/fpa/fpa2bv_model_converter.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/model"
	modelfpa "github.com/glycerine/g3/pkg/model/fpa"
)

type tacticFPA2BVModelConverter struct {
	tacticConverterBase
	manager   *ast.ASTManager
	converter *modelfpa.Converter
}

// NewFPA2BVModelConverter snapshots the conversion symbols accumulated by the
// FPA-to-BV rewriter.
func NewFPA2BVModelConverter(m *ast.ASTManager, source *astfpa.Converter) (*tacticFPA2BVModelConverter, error) {
	if m == nil || source == nil {
		return nil, ast.ErrASTHandle
	}
	converter, err := modelfpa.NewConverter(m, source)
	if err != nil {
		return nil, err
	}
	result := &tacticFPA2BVModelConverter{manager: m, converter: converter}
	if err = result.init(result, m.Budget(), uint64(reflect.TypeOf(*result).Size())); err != nil {
		return nil, errors.Join(err, converter.Close())
	}
	return result, nil
}

func newTranslatedFPA2BVModelConverter(m *ast.ASTManager, converter *modelfpa.Converter) (*tacticFPA2BVModelConverter, error) {
	result := &tacticFPA2BVModelConverter{manager: m, converter: converter}
	if err := result.init(result, m.Budget(), uint64(reflect.TypeOf(*result).Size())); err != nil {
		return nil, errors.Join(err, converter.Close())
	}
	return result, nil
}

func (c *tacticFPA2BVModelConverter) ApplyModel(out *ModelRef, goal uint32) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if goal != 0 {
		return errTacticModelConverterGoal
	}
	source, err := out.Get()
	if err != nil {
		return err
	}
	if source == nil || source.Manager() != c.manager || source.DerivedModel() == nil {
		return ast.ErrASTHandle
	}
	target, err := model.NewConverterModel(c.manager)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			result = errors.Join(result, target.Close())
		}
	}()
	seen, err := c.converter.ConvertModel(source, target.Core())
	if err != nil {
		return err
	}
	constants, err := source.NumConstants()
	if err != nil {
		return err
	}
	for i := uint32(0); i < constants; i++ {
		declaration, err := source.Constant(i)
		if err != nil {
			return err
		}
		if _, consumed := seen[declaration]; consumed {
			continue
		}
		value, err := source.ConstantInterpretation(declaration)
		if err != nil {
			return err
		}
		if err = target.Core().RegisterConstant(declaration, value); err != nil {
			return err
		}
	}
	for i := uint32(0); i < source.NumFunctionDeclarations(); i++ {
		declaration, err := source.Function(i)
		if err != nil {
			return err
		}
		if _, consumed := seen[declaration]; consumed {
			continue
		}
		interpretation, err := source.FunctionInterpretation(declaration)
		if err != nil {
			return err
		}
		if interpretation == nil {
			return ast.ErrASTPending
		}
		if err = target.Core().CopyFunctionInterpretation(declaration, interpretation); err != nil {
			return err
		}
	}
	if err = target.CopyUniverseInterpretations(source.DerivedModel()); err != nil {
		return err
	}
	if err = out.Set(target.Core()); err != nil {
		published = out.holds(target.Core())
		return err
	}
	published = true
	return nil
}

func (c *tacticFPA2BVModelConverter) ApplyUnindexed(out *ModelRef) error {
	return c.ApplyModel(out, 0)
}
func (c *tacticFPA2BVModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error {
	return c.open()
}
func (c *tacticFPA2BVModelConverter) Cancel() error { return c.open() }

func (c *tacticFPA2BVModelConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(out, "(fpa2bv-model-converter"); err != nil {
		return err
	}
	if err := c.converter.Display(out); err != nil {
		return err
	}
	_, err := io.WriteString(out, ")")
	return err
}

func (c *tacticFPA2BVModelConverter) Translate(translation *ast.ASTTranslation) (ModelConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	translated, err := c.converter.Translate(translation)
	if err != nil {
		return nil, err
	}
	return newTranslatedFPA2BVModelConverter(translation.To(), translated)
}

func (c *tacticFPA2BVModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	if c.converter != nil {
		if err := c.converter.Close(); err != nil {
			return err
		}
		c.converter = nil
	}
	c.release()
	return nil
}
