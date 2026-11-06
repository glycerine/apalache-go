package cmd_context

// Source: src/cmd_context/cmd_context.cpp complete_model and validate_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// completeModel applies the command-context model.completion option to the
// current successful check result. ModelCore owns the added interpretations.
func (c *Context) completeModel() (resultErr error) {
	enabled, err := c.config.Globals.SourceGetValueName("model.completion")
	if err != nil || enabled != "true" {
		return err
	}
	md, err := c.availableCheckModel()
	if err != nil {
		return err
	}
	if md == nil {
		return nil
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	derived := md.Derived
	if derived == nil {
		return ast.ErrASTHandle
	}

	seenSorts := make(map[ast.ASTHandle]bool)
	for _, declaration := range c.sortTrail {
		sort := declaration.handle
		if seenSorts[sort] {
			continue
		}
		seenSorts[sort] = true
		uninterpreted, err := c.manager.IsUninterpretedSort(sort)
		if err != nil || !uninterpreted {
			if err != nil {
				return err
			}
			continue
		}
		present, err := derived.HasUninterpretedSort(sort)
		if err != nil || present {
			if err != nil {
				return err
			}
			continue
		}
		value, err := derived.GetSomeValue(sort)
		if err != nil {
			return err
		}
		if err = derived.RegisterUniverse(sort, []ast.ASTHandle{value}); err != nil {
			return err
		}
	}

	count := md.NumFunctionDeclarations()
	for i := uint32(0); i < count; i++ {
		declaration, err := md.Function(i)
		if err != nil {
			return err
		}
		interpretation, err := md.FunctionInterpretation(declaration)
		if err != nil || interpretation == nil {
			if err != nil {
				return err
			}
			continue
		}
		partial, err := interpretation.IsPartial()
		if err != nil || !partial {
			if err != nil {
				return err
			}
			continue
		}
		rangeSort, err := c.manager.GetRange(declaration)
		if err != nil {
			return err
		}
		value, err := derived.GetSomeValue(rangeSort)
		if err != nil {
			return err
		}
		if err = interpretation.SetElse(value); err != nil {
			return err
		}
	}

	seenFunctions := make(map[ast.ASTHandle]bool)
	for _, declaration := range c.functionTrail {
		function := declaration.handle
		if seenFunctions[function] {
			continue
		}
		seenFunctions[function] = true
		present, err := md.HasInterpretation(function)
		if err != nil || present {
			if err != nil {
				return err
			}
			continue
		}
		rangeSort, err := c.manager.GetRange(function)
		if err != nil {
			return err
		}
		value, err := derived.GetSomeValue(rangeSort)
		if err != nil {
			return err
		}
		arity, err := c.manager.GetArity(function)
		if err != nil {
			return err
		}
		if arity == 0 {
			if err = md.RegisterConstant(function, value); err != nil {
				return err
			}
			continue
		}
		interpretation, err := model.NewFunctionInterpretation(c.manager, arity)
		if err != nil {
			return err
		}
		if err = interpretation.SetElse(value); err != nil {
			return errors.Join(err, interpretation.Close())
		}
		if err = md.RegisterFunction(function, interpretation); err != nil {
			return errors.Join(err, interpretation.Close())
		}
	}
	return nil
}

// validateModel checks ground assertions against a successful solver model
// when the native model_validate option is enabled. Array expressions and
// datatype accessors are exempt because their model evaluation is incomplete.
func (c *Context) validateModel() (resultErr error) {
	enabled, err := c.config.Globals.SourceGetValueName("model_validate")
	if err != nil || enabled != "true" || !c.checked || c.status != lbool.True {
		return err
	}
	md, err := c.availableCheckModel()
	if err != nil {
		return err
	}
	if md == nil {
		return nil
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	if err = evaluator.SetExpandArrayEqualities(false); err != nil {
		return err
	}
	invalid := false
	for _, assertion := range c.assertions {
		ground, err := c.manager.IsGround(assertion.expression)
		if err != nil || !ground {
			if err != nil {
				return err
			}
			continue
		}
		value, err := evaluator.EvaluateValue(assertion.expression)
		if err != nil {
			return err
		}
		h, err := c.manager.RefGet(value)
		if err == nil {
			quantified, e := c.manager.HasQuantifiers(h)
			if e != nil {
				err = e
			} else if !quantified {
				underSpecified, e := c.hasUnderSpecifiedModelOps(assertion.expression)
				if e == nil && !underSpecified {
					underSpecified, e = c.hasUnderSpecifiedModelOps(h)
				}
				if e != nil {
					err = e
				} else if !underSpecified && !c.manager.IsTrue(h) {
					invalid = true
				}
			}
		}
		err = errors.Join(err, c.manager.DelRef(&value))
		if err != nil {
			return err
		}
	}
	if invalid {
		return errors.New("an invalid model was generated")
	}
	return nil
}

func (c *Context) availableCheckModel() (*model.ModelCore, error) {
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	if !state.Model || !c.checked || c.status != lbool.True && c.status != lbool.Undef {
		return nil, nil
	}
	return c.checkSatResult().AcquireModel()
}

func (c *Context) hasUnderSpecifiedModelOps(root ast.ASTHandle) (bool, error) {
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	arrayFamily, err := c.manager.ArrayFamilyID()
	if err != nil {
		return false, err
	}
	dtFamily, err := c.manager.DatatypeFamilyID()
	if err != nil {
		return false, err
	}
	for len(stack) != 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		kind, err := c.manager.GetKind(n)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := c.manager.GetFamilyID(n)
		if err != nil {
			return false, err
		}
		declKind, err := c.manager.GetDeclKind(n)
		if err != nil {
			return false, err
		}
		if family == dtFamily && declKind == ast.ASTDatatypeAccessor || family == arrayFamily && (declKind == ast.ASTArrayOpAsArray || declKind == ast.ASTArrayOpStore || declKind == ast.ASTArrayOpMap || declKind == ast.ASTArrayOpConst) {
			return true, nil
		}
		count, err := c.manager.GetNumArgs(n)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			arg, err := c.manager.GetArg(n, i)
			if err != nil {
				return false, err
			}
			stack = append(stack, arg)
		}
	}
	return false, nil
}
