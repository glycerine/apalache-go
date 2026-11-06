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

func (c *Context) substituteModelValues(modelCore *model.ModelCore, expression ast.ASTHandle, sources []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	if modelCore == nil || modelCore.DerivedModel() == nil {
		return result, ast.ErrASTDomain
	}
	substitution, err := c.manager.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelExprSubstitution(&substitution)) }()
	config := c.owner.contexts.config
	replacer, err := rewriter.NewSimplifyingExprReplacer(c.manager, config.Params, config.Globals, c.owner.contexts.params)
	if err != nil {
		if replacer != nil {
			err = errors.Join(err, replacer.Close())
		}
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	valueRef, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&valueRef)) }()
	seen := make(map[ast.ASTHandle]bool, len(sources))
	for _, source := range sources {
		if seen[source] {
			continue
		}
		seen[source] = true
		evaluated, x := c.implicant.EvalExpr(modelCore.DerivedModel(), source)
		if x != nil {
			return result, x
		}
		valueHandle, x := c.manager.RefGet(evaluated)
		if x == nil {
			x = c.manager.SetRef(valueRef, valueHandle)
		}
		x = errors.Join(x, c.manager.DelRef(&evaluated))
		if x != nil {
			return result, x
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
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&references[index]))
		}
	}()
	// flatten_and owns every entry, including newly constructed negations.
	pinForms := func(forms []ast.ASTHandle) error {
		for _, form := range forms {
			ref, err := c.manager.NewRef(form)
			if err != nil {
				return err
			}
			references = append(references, ref)
		}
		return nil
	}
	if err = pinForms(forms); err != nil {
		return err
	}
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
	variableRefs := make([]ast.ASTRef, 0, len(eliminated))
	defer func() {
		for index := len(variableRefs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&variableRefs[index]))
		}
	}()
	for _, variable := range eliminated {
		ref, x := c.manager.NewRef(variable)
		if x != nil {
			return x
		}
		variableRefs = append(variableRefs, ref)
	}
	formulaRef, err := c.normalizeProjectedPhi(forms, &variableRefs, modelCore.DerivedModel())
	if err != nil {
		return err
	}
	references = append(references, formulaRef)
	eliminated = eliminated[:0]
	for _, reference := range variableRefs {
		variable, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		eliminated = append(eliminated, variable)
	}
	formula, err := c.manager.RefGet(formulaRef)
	if err != nil {
		return err
	}
	forms, err = c.manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return err
	}
	if err = pinForms(forms); err != nil {
		return err
	}
	projected := make([]ast.ASTHandle, 0, len(forms))
	for _, formula := range forms {
		reference, x := c.substituteModelValues(modelCore, formula, eliminated)
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
	if err = pinForms(projected); err != nil {
		return err
	}

	childStates := make([][]ast.ASTHandle, len(rule.predecessors))
	valueRef, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&valueRef)) }()
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
				equalityRef, y := c.manager.NewRef(ast.ASTHandle{})
				if y != nil {
					return y
				}
				references = append(references, equalityRef)
				equality, y := c.manager.MkEq(variable, value)
				if !equality.IsNull() {
					y = errors.Join(y, c.manager.SetRef(equalityRef, equality))
				}
				if y != nil {
					return y
				}
				if y = retainChild(index, equality); y != nil {
					return y
				}
				replaced = append(replaced, variable)
			}
		}
		reduced, x := c.substituteModelValues(modelCore, literal, replaced)
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
		if err = c.addProjectedChild(node, predecessor, childStates[predecessorIndex]); err != nil {
			return err
		}
	}
	return node.CheckPreClosed()
}

// addProjectedChild owns the rewritten cube until the model node retains it.
func (c *Context) addProjectedChild(node *ModelNode, predecessor *PredicateTransformer, states []ast.ASTHandle) (resultErr error) {
	cubeRef, err := c.owner.AndRef(states)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&cubeRef)) }()
	cube, err := c.manager.RefGet(cubeRef)
	if err != nil {
		return err
	}
	child, err := NewModelNode(node, cube, predecessor, node.level-1)
	if err != nil {
		return fmt.Errorf("muz/pdr: projected model-node construction: %w", err)
	}
	c.nodes++
	if child.depth > c.maxDepth {
		c.maxDepth = child.depth
	}
	return c.search.AddLeaf(child)
}

// normalizeProjectedPhi follows create_children: Boolean conjunction, QE,
// optional disequality reduction, then the fixedpoint theory rewriter.
func (c *Context) normalizeProjectedPhi(forms []ast.ASTHandle, vars *[]ast.ASTRef, md *model.Model) (result ast.ASTRef, resultErr error) {
	result, resultErr = c.owner.AndRef(forms)
	if resultErr != nil {
		return result, resultErr
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&result))
		}
	}()
	if c.lite != nil {
		if resultErr = c.lite.EliminateConstants(vars, result); resultErr != nil {
			return result, fmt.Errorf("muz/pdr: QE-lite projection: %w", resultErr)
		}
	}
	if !c.useModel && c.implicant != nil && md != nil {
		formula, err := c.manager.RefGet(result)
		if err != nil {
			return result, err
		}
		reduced, err := c.reduceDisequalities(md, 3, formula)
		if err != nil {
			return result, fmt.Errorf("muz/pdr: disequality reduction: %w", err)
		}
		formula, err = c.manager.RefGet(reduced)
		if err == nil {
			err = c.manager.SetRef(result, formula)
		}
		err = errors.Join(err, c.owner.releaseNotAndRef(&reduced))
		if err != nil {
			return result, err
		}
	}
	// Nil-Globals contexts are a supported partial Go configuration and have
	// no parameter service from which to construct the source rewriter.
	if err := c.ensureProjectionTheory(); err != nil {
		return result, err
	}
	if c.projectionTheory == nil {
		return result, nil
	}
	formula, err := c.manager.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = c.projectionTheory.Rewrite(formula, result); err != nil {
		return result, err
	}
	return result, nil
}

// ensureProjectionTheory also permits retry after a partially constructed
// rewriter refused cleanup during an earlier attempt.
func (c *Context) ensureProjectionTheory() error {
	config := c.owner.contexts.config
	if config.Globals == nil {
		return nil
	}
	if c.projectionTheory != nil && !c.projectionTheoryReady {
		if err := c.projectionTheory.Close(); err != nil {
			return err
		}
		c.projectionTheory = nil
	}
	if c.projectionTheory == nil {
		var err error
		c.projectionTheory, err = rewriter.NewTheoryRewriter(c.manager, config.Params, config.Globals, nil)
		if err != nil {
			return err
		}
		c.projectionTheoryReady = true
	}
	return nil
}
