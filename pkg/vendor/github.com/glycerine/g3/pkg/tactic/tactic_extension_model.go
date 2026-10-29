package tactic

// Source: src/tactic/extension_model_converter.cpp:26-27,49-85;
// src/tactic/extension_model_converter.h:27-50;
// src/tactic/model_converter.h:30-41; src/util/ref_vector.h:96-100.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// The source reconstruction walks eliminated definitions in reverse order.
type tacticExtensionModelConverter struct {
	tacticConverterBase
	manager    *ast.ASTManager
	services   model.ModelEvaluatorConfig
	vars, defs []ast.ASTHandle
	evaluator  *model.ModelEvaluator
	value      ast.ASTRef
	function   *model.FunctionInterpretation
}

// ExtensionModelConverter reconstructs the concrete definitions eliminated by
// a tactic. Its model-converter ownership contract is unchanged.
type ExtensionModelConverter = tacticExtensionModelConverter

func NewExtensionModel(m *ast.ASTManager, services model.ModelEvaluatorConfig) (*tacticExtensionModelConverter, error) {
	if m == nil || services.Context == nil || services.Globals == nil || services.Memory == nil || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	if services.Context.SymbolsField != m.Symbols || services.Globals.Context != services.Context {
		return nil, ast.ErrASTHandle
	}
	// Source constructs its lexical evaluator with default parameters.
	services.Params = nil
	c := &tacticExtensionModelConverter{manager: m, services: services}
	if e := c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); e != nil {
		return nil, e
	}
	return c, nil
}
func (c *tacticExtensionModelConverter) Insert(d, def ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	m := c.manager
	k, e := m.GetKind(d)
	if e != nil {
		return e
	}
	if k != ast.ASTFuncDecl {
		return ast.ErrASTDomain
	}
	k, e = m.GetKind(def)
	if e != nil {
		return e
	}
	if k != ast.ASTApp && k != ast.ASTVar && k != ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	if e = m.IncRef(d); e != nil {
		return e
	}
	c.vars = append(c.vars, d)
	if e = m.IncRef(def); e != nil {
		return e
	}
	c.defs = append(c.defs, def)
	return nil
}
func (c *tacticExtensionModelConverter) drainInvocation() error {
	if c.function != nil {
		if e := c.function.Close(); e != nil {
			return e
		}
		c.function = nil
	}
	if c.value != (ast.ASTRef{}) {
		if e := c.manager.DelRef(&c.value); e != nil {
			return e
		}
	}
	if c.evaluator != nil {
		if e := c.evaluator.Close(); e != nil {
			return e
		}
		c.evaluator = nil
	}
	return nil
}
func (c *tacticExtensionModelConverter) ApplyModel(out *ModelRef, goal uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if tacticSourceDebug && goal != 0 {
		return errTacticModelConverterGoal
	}
	if c.evaluator != nil || c.function != nil || c.value != (ast.ASTRef{}) {
		return ErrModelConverterOwner
	}
	if len(c.vars) != len(c.defs) {
		return ast.ErrASTDomain
	}
	md, e := out.Get()
	if e != nil {
		return e
	}
	if md == nil || md.Manager() != c.manager {
		return ast.ErrASTHandle
	}
	if e = c.traceExtensionModel(md, 49); e != nil {
		return e
	}
	defer func() { err = errors.Join(err, c.drainInvocation()) }()
	c.evaluator, e = model.NewModelEvaluator(md, c.services)
	if e != nil {
		return e
	}
	if e = c.evaluator.SetModelCompletion(true); e != nil {
		return e
	}
	if e = c.evaluator.SetExpandArrayEqualities(false); e != nil {
		return e
	}
	c.value, e = c.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	for i := len(c.vars); i > 0; {
		i--
		if e = c.evaluator.Evaluate(c.defs[i], c.value); e != nil {
			return e
		}
		value, e := c.manager.RefGet(c.value)
		if e != nil {
			return e
		}
		d := c.vars[i]
		if e = c.traceExtensionDefinition(d, c.defs[i], value); e != nil {
			return e
		}
		arity, e := c.manager.GetArity(d)
		if e != nil {
			return e
		}
		if arity == 0 {
			if e = md.RegisterConstant(d, value); e != nil {
				return e
			}
		} else {
			c.function, e = model.NewExtensionFunctionInterpretation(c.manager, arity)
			if e != nil {
				return e
			}
			if e = c.function.SetElse(value); e != nil {
				return e
			}
			if e = md.RegisterFunction(d, c.function); e != nil {
				return e
			}
			c.function = nil
		}
	}
	return c.traceExtensionModel(md, 71)
}

func (c *tacticExtensionModelConverter) Translate(translator *ast.ASTTranslation) (_ ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if translator == nil || translator.From() != c.manager || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := NewExtensionModel(translator.To(), c.services)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, result.Close())
		}
	}()
	for _, declaration := range c.vars {
		translated, e := translator.Translate(declaration)
		if e != nil {
			return nil, e
		}
		if e = result.manager.IncRef(translated); e != nil {
			return nil, e
		}
		result.vars = append(result.vars, translated)
	}
	for _, definition := range c.defs {
		translated, e := translator.Translate(definition)
		if e != nil {
			return nil, e
		}
		if e = result.manager.IncRef(translated); e != nil {
			return nil, e
		}
		result.defs = append(result.defs, translated)
	}
	published = true
	return result, nil
}

// Preserve the inherited source unindexed and labels no-op operations.
func (c *tacticExtensionModelConverter) ApplyUnindexed(_ *ModelRef) error           { return c.open() }
func (c *tacticExtensionModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error { return c.open() }
func (c *tacticExtensionModelConverter) Cancel() error                              { return c.open() }
func (c *tacticExtensionModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	if e := c.drainInvocation(); e != nil {
		return e
	}
	// Reverse member order: expression references before declaration references;
	// each original ref_vector releases entries in forward order.
	for _, vector := range []*[]ast.ASTHandle{&c.defs, &c.vars} {
		for i, h := range *vector {
			if !h.IsNull() {
				consumed, e := c.manager.DecRefConsumed(h)
				if consumed {
					(*vector)[i] = ast.ASTHandle{}
				}
				if e != nil {
					return e
				}
				if !consumed {
					return ast.ErrASTPending
				}
			}
		}
		*vector = nil
	}
	c.release()
	return nil
}
