package pdr

// Source: context::create_children model substitution and predecessor-literal
// partitioning in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (c *Context) substituteModelValues(modelCore *model.ModelCore, transformer *PredicateTransformer, expression ast.ASTHandle, sources []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	substitution, err := c.manager.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelExprSubstitution(&substitution)) }()
	replacer, err := rewriter.NewDefaultExprReplacer(c.manager)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	valueRef, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&valueRef)) }()
	seen := make(map[ast.ASTHandle]bool, len(sources))
	for _, source := range sources {
		if seen[source] {
			continue
		}
		seen[source] = true
		found, x := transformer.modelConstant(modelCore, source, valueRef)
		if x != nil {
			return result, x
		}
		if !found {
			return result, ast.ErrASTDomain
		}
		value, x := c.manager.RefGet(valueRef)
		if x != nil {
			return result, x
		}
		if x = c.manager.ExprSubstitutionInsert(substitution, source, value, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
			return result, x
		}
	}
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return result, err
	}
	result, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = replacer.Rewrite(expression, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, c.manager.DelRef(&result))
	}
	return result, nil
}

func (c *Context) createChildrenProjected(node *ModelNode) (resultErr error) {
	proofsEnabled, err := c.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofsEnabled {
		if err = c.manager.ToggleProofMode(ast.ASTProofDisabled); err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, c.manager.ToggleProofMode(ast.ASTProofEnabled)) }()
	}
	modelCore, found, err := node.Model()
	if err != nil {
		return err
	}
	if !found {
		return ast.ErrASTDomain
	}
	rule, err := node.transformer.selectRule(modelCore)
	if err != nil {
		return err
	}
	if err = node.SetRule(rule.rule); err != nil {
		return err
	}
	node.selected = rule

	eliminated := make([]ast.ASTHandle, 0, len(node.transformer.signature)+len(rule.auxiliaries))
	for _, signature := range node.transformer.signature {
		declaration, x := c.owner.OldToNew(signature, 0)
		if x != nil {
			return x
		}
		application, x := c.manager.MkApp(declaration, nil)
		if x != nil {
			return x
		}
		eliminated = append(eliminated, application)
	}
	eliminated = append(eliminated, rule.auxiliaries...)

	forms, err := c.manager.FlattenAnd([]ast.ASTHandle{rule.transition, node.state})
	if err != nil {
		return err
	}
	references := make([]ast.ASTRef, 0, len(forms))
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&references[index]))
		}
	}()
	if derived := modelCore.DerivedModel(); c.implicant != nil && derived != nil {
		var minimized []ast.ASTRef
		var x error
		if c.useModel {
			minimized, x = c.implicant.MinimizeModel(forms, derived)
		} else {
			minimized, x = c.implicant.MinimizeLiterals(forms, derived)
		}
		if x != nil {
			return fmt.Errorf("muz/pdr: implicant minimization: %w", x)
		}
		references = append(references, minimized...)
		forms = forms[:0]
		for _, reference := range minimized {
			formula, y := c.manager.RefGet(reference)
			if y != nil {
				return y
			}
			forms = append(forms, formula)
		}
	}
	if c.lite != nil && len(eliminated) != 0 {
		formula, x := c.manager.MkAndArray(forms)
		if x != nil {
			return x
		}
		formulaRef, x := c.manager.NewRef(formula)
		if x != nil {
			return x
		}
		references = append(references, formulaRef)
		variableRefs := make([]ast.ASTRef, 0, len(eliminated))
		defer func() {
			for index := len(variableRefs) - 1; index >= 0; index-- {
				resultErr = errors.Join(resultErr, c.manager.DelRef(&variableRefs[index]))
			}
		}()
		for _, variable := range eliminated {
			reference, y := c.manager.NewRef(variable)
			if y != nil {
				return y
			}
			variableRefs = append(variableRefs, reference)
		}
		if x = c.lite.EliminateConstants(&variableRefs, formulaRef); x != nil {
			return fmt.Errorf("muz/pdr: QE-lite projection: %w", x)
		}
		eliminated = eliminated[:0]
		for _, reference := range variableRefs {
			variable, y := c.manager.RefGet(reference)
			if y != nil {
				return y
			}
			eliminated = append(eliminated, variable)
		}
		formula, x = c.manager.RefGet(formulaRef)
		if x != nil {
			return x
		}
		if derived := modelCore.DerivedModel(); !c.useModel && c.implicant != nil && derived != nil {
			reduced, y := c.reduceDisequalities(derived, 3, formula)
			if y != nil {
				return fmt.Errorf("muz/pdr: disequality reduction: %w", y)
			}
			references = append(references, reduced)
			formula, y = c.manager.RefGet(reduced)
			if y != nil {
				return y
			}
		}
		forms, x = c.manager.FlattenAnd([]ast.ASTHandle{formula})
		if x != nil {
			return x
		}
	}
	projected := make([]ast.ASTHandle, 0, len(forms))
	for _, formula := range forms {
		reference, x := c.substituteModelValues(modelCore, node.transformer, formula, eliminated)
		if x != nil {
			return fmt.Errorf("muz/pdr: projected model substitution: %w", x)
		}
		references = append(references, reference)
		handle, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		projected = append(projected, handle)
	}
	projected, err = c.manager.FlattenAnd(projected)
	if err != nil {
		return err
	}

	childStates := make([][]ast.ASTHandle, len(rule.predecessors))
	valueRef, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&valueRef)) }()
	retainChild := func(index uint32, formula ast.ASTHandle) error {
		if index >= uint32(len(childStates)) {
			return ast.ErrASTDomain
		}
		converted, x := c.owner.FormulaOldToNew(formula, index, true)
		if x != nil {
			return fmt.Errorf("muz/pdr: child-state renaming: %w", x)
		}
		reference, x := c.manager.NewRef(converted)
		if x != nil {
			return x
		}
		references = append(references, reference)
		childStates[index] = append(childStates[index], converted)
		return nil
	}
	for _, literal := range projected {
		indices, x := c.owner.mux.CollectIndices(literal)
		if x != nil {
			return fmt.Errorf("muz/pdr: projected state-index collection: %w", x)
		}
		if len(indices) == 0 {
			continue
		}
		if len(indices) == 1 {
			if x = retainChild(indices[0], literal); x != nil {
				return x
			}
			continue
		}
		variables, x := c.owner.mux.CollectVariables(literal)
		if x != nil {
			return fmt.Errorf("muz/pdr: projected state-variable collection: %w", x)
		}
		replaced := make([]ast.ASTHandle, 0)
		for _, index := range indices[1:] {
			if index >= uint32(len(variables)) || index >= uint32(len(rule.predecessors)) {
				return ast.ErrASTDomain
			}
			for _, variable := range variables[index] {
				found, y := node.transformer.modelConstant(modelCore, variable, valueRef)
				if y != nil {
					return y
				}
				if !found {
					return ast.ErrASTDomain
				}
				value, y := c.manager.RefGet(valueRef)
				if y != nil {
					return y
				}
				equality, y := c.manager.MkEq(variable, value)
				if y != nil {
					return y
				}
				if y = retainChild(index, equality); y != nil {
					return y
				}
				replaced = append(replaced, variable)
			}
		}
		reduced, x := c.substituteModelValues(modelCore, node.transformer, literal, replaced)
		if x != nil {
			return fmt.Errorf("muz/pdr: coupled-state model substitution: %w", x)
		}
		references = append(references, reduced)
		reducedHandle, x := c.manager.RefGet(reduced)
		if x != nil {
			return x
		}
		if x = retainChild(indices[0], reducedHandle); x != nil {
			return x
		}
	}

	for predecessorIndex, predecessor := range rule.predecessors {
		cube, x := c.manager.MkAndArray(childStates[predecessorIndex])
		if x != nil {
			return x
		}
		child, x := NewModelNode(node, cube, predecessor, node.level-1)
		if x != nil {
			return fmt.Errorf("muz/pdr: projected model-node construction: %w", x)
		}
		c.nodes++
		if child.depth > c.maxDepth {
			c.maxDepth = child.depth
		}
		if x = c.search.AddLeaf(child); x != nil {
			return x
		}
	}
	return node.CheckPreClosed()
}
