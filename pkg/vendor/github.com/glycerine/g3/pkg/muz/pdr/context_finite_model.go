package pdr

// Source: pdr_context.cpp get_level_property and
// pdr_manager.cpp inductive_property::to_model.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *Context) finiteFrameModel(level uint32, queryDeclaration ast.ASTHandle) (result *model.Model, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if level == InfinityLevel || c.queryDecl != queryDeclaration {
		return nil, ast.ErrASTDomain
	}
	result, err := model.NewModel(c.manager)
	if err != nil {
		return nil, err
	}
	owner := &pdrModelExportOwner{manager: c.manager, model: result}
	defer func() {
		if resultErr == nil {
			owner.model = nil
		} else {
			result = nil
			resultErr = owner.dispose(resultErr)
		}
	}()
	pin := func(handle ast.ASTHandle) error {
		reference, err := c.manager.NewRef(handle)
		if err == nil {
			owner.references = append(owner.references, reference)
		}
		return err
	}
	abstractor, err := ast.NewExprAbstractor(c.manager)
	if err != nil {
		return result, err
	}
	owner.abstractors = append(owner.abstractors, abstractor)
	for _, declaration := range c.order {
		if declaration == queryDeclaration {
			continue
		}
		transformer := c.transformers[declaration]
		bodyRef, x := transformer.FormulasRef(level, false)
		var body ast.ASTHandle
		if x == nil {
			owner.references = append(owner.references, bodyRef)
			body, x = c.manager.RefGet(bodyRef)
		}
		if x == nil {
			var converted ast.ASTRef
			converted, x = c.owner.FormulaNewToOldRef(body, 0, false)
			if converted != (ast.ASTRef{}) {
				owner.references = append(owner.references, converted)
			}
			if x == nil {
				body, x = c.manager.RefGet(converted)
			}
		}
		if x != nil {
			return result, x
		}
		fixed, x := c.fixupInductiveClausesRef(body)
		if fixed != (ast.ASTRef{}) {
			owner.references = append(owner.references, fixed)
		}
		if x != nil {
			return result, x
		}
		body, x = c.manager.RefGet(fixed)
		if x != nil {
			return result, x
		}
		arguments := make([]ast.ASTHandle, len(transformer.signature))
		for index := range arguments {
			arguments[index], x = c.manager.MkApp(transformer.signature[len(arguments)-index-1], nil)
			if x == nil {
				x = pin(arguments[index])
			}
			if x != nil {
				return result, x
			}
		}
		abstracted, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return result, x
		}
		owner.references = append(owner.references, abstracted)
		if x = abstractor.Abstract(0, arguments, body, abstracted); x == nil {
			body, x = c.manager.RefGet(abstracted)
		}
		if x != nil {
			return result, x
		}
		if len(arguments) == 0 {
			x = result.Core().RegisterConstant(declaration, body)
		} else {
			var interpretation *model.FunctionInterpretation
			interpretation, x = model.NewFunctionInterpretation(c.manager, uint32(len(arguments)))
			if interpretation != nil {
				owner.interpretations = append(owner.interpretations, interpretation)
			}
			if x == nil {
				x = interpretation.SetElse(body)
			}
			if x == nil {
				x = result.Core().RegisterFunction(declaration, interpretation)
				if x == nil {
					owner.interpretations = owner.interpretations[:len(owner.interpretations)-1]
				}
			}
		}
		if x != nil {
			return result, x
		}
	}
	if err = c.traceInductiveModel(result); err != nil {
		return result, err
	}
	if err = owner.releaseTemporaries(); err != nil {
		return result, err
	}
	return result, nil
}
