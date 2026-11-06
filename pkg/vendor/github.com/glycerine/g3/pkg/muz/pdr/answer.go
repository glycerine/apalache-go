package pdr

// Source: model_search::get_trace, context::get_answer and
// context::mk_sat_answer and context::mk_unsat_answer in
// src/muz/pdr/pdr_context.cpp, plus inductive_property::to_expr in
// src/muz/pdr/pdr_manager.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) instantiateTraceExpression(node *ModelNode, rule *normalizedRule, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	conjuncts, err := c.manager.FlattenAnd([]ast.ASTHandle{node.state})
	if err != nil {
		return result, err
	}
	values := make(map[ast.ASTHandle]ast.ASTHandle)
	truth, err := c.manager.MkTrue()
	if err != nil {
		return result, err
	}
	falsity, err := c.manager.MkFalse()
	if err != nil {
		return result, err
	}
	for _, conjunct := range conjuncts {
		kind, x := c.manager.GetKind(conjunct)
		if x != nil {
			return result, x
		}
		matched := false
		if kind == ast.ASTApp {
			family, y := c.manager.GetFamilyID(conjunct)
			if y != nil {
				return result, y
			}
			operation, y := c.manager.GetDeclKind(conjunct)
			if y != nil {
				return result, y
			}
			if family == c.manager.BasicFamilyID() && (operation == ast.ASTOpEq || operation == ast.ASTOpIff) {
				left, y := c.manager.GetArg(conjunct, 0)
				if y != nil {
					return result, y
				}
				right, y := c.manager.GetArg(conjunct, 1)
				if y != nil {
					return result, y
				}
				rightValue, y := c.manager.IsValue(right)
				if y != nil {
					return result, y
				}
				if rightValue {
					values[left] = right
					matched = true
				} else if leftValue, y := c.manager.IsValue(left); y != nil {
					return result, y
				} else if leftValue {
					values[right] = left
					matched = true
				}
			}
		}
		if matched {
			continue
		}
		if negated, argument, x := c.manager.IsNot(conjunct); x != nil {
			return result, x
		} else if negated {
			values[argument] = falsity
		} else {
			values[conjunct] = truth
		}
	}
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return result, err
	}
	if err = used.Collect(expression); err != nil {
		return result, err
	}
	arguments := make([]ast.ASTHandle, len(rule.instances))
	references := make([]ast.ASTRef, 0, len(arguments))
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&references[index]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	for index, instance := range rule.instances {
		if value, found := values[instance]; found {
			arguments[index] = value
		} else {
			sort := used.Contains(uint32(index))
			if !instance.IsNull() {
				sort, err = c.manager.GetSort(instance)
			}
			if err != nil {
				return result, err
			}
			if sort.IsNull() {
				continue
			}
			arguments[index], err = c.manager.MkVar(uint32(index), sort)
		}
		if err != nil {
			return result, err
		}
		if arguments[index].IsNull() {
			continue
		}
		reference, x := c.manager.NewRef(arguments[index])
		if x != nil {
			return result, x
		}
		references = append(references, reference)
	}
	substitution, err := rewriter.NewVarSubstitution(c.manager, false)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	result, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = substitution.Substitute(expression, arguments, result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Context) selectedTraceRule(node *ModelNode) (result *normalizedRule, resultErr error) {
	if node.selected != nil {
		return node.selected, nil
	}
	// Source model_node::get_rule first selects from the witness already
	// stored by the search. Initial-rule export does not rerun that search
	// or replace its model.
	if node.model != nil {
		rule, err := node.transformer.selectRule(node.model)
		if err != nil {
			return nil, err
		}
		if len(rule.predecessors) == 0 {
			if err = node.SetRule(rule.rule); err != nil {
				return nil, err
			}
			node.selected = rule
			return rule, nil
		}
	}
	initial := make([]*normalizedRule, 0)
	tags := make([]ast.ASTHandle, 0)
	for _, rule := range node.transformer.rules {
		if len(rule.predecessors) != 0 {
			continue
		}
		initial = append(initial, rule)
		if !rule.tag.IsNull() {
			tags = append(tags, rule.tag)
		}
	}
	// A constraint-cache hit need not carry a model. Even a unique initial
	// rule must satisfy this state; recover its witness with the source
	// level-zero/tag selection instead of assuming it explains the cache hit.
	if len(initial) == 0 || len(tags) != len(initial) {
		return nil, ast.ErrASTDomain
	}
	selection, err := c.manager.MkOrArray(tags)
	if err != nil {
		return nil, err
	}
	// Proof export temporarily enables proof construction, but this query
	// must use the proof mode in which the live PDR solver was initialized.
	restoreProofMode, err := c.enterPDRProofMode()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, restoreProofMode()) }()
	check, err := node.transformer.solver.CheckAtLevel(0, []ast.ASTHandle{selection, node.state}, false, true)
	if err != nil {
		return nil, err
	}
	if check.Status != lbool.True || check.Model == nil {
		if check.Model != nil {
			err = check.Model.DecRef()
		}
		return nil, errors.Join(ast.ErrASTDomain, err)
	}
	rule, selectErr := node.transformer.selectInitialRule(check.Model)
	if selectErr == nil && node.model == nil {
		// The model-less cache recovery is a Go extension. Source fallback
		// selection leaves an existing witness model in place.
		selectErr = node.SetModel(check.Model)
	}
	if selectErr == nil {
		selectErr = node.SetRule(rule.rule)
	}
	if selectErr == nil {
		node.selected = rule
	}
	selectErr = errors.Join(selectErr, check.Model.DecRef())
	if selectErr != nil {
		return nil, selectErr
	}
	return rule, nil
}

func traceTransformerReaches(current, target *PredicateTransformer, seen map[*PredicateTransformer]bool) bool {
	if seen[current] {
		return false
	}
	seen[current] = true
	for _, rule := range current.rules {
		for _, predecessor := range rule.predecessors {
			if predecessor == target || traceTransformerReaches(predecessor, target, seen) {
				return true
			}
		}
	}
	return false
}

func traceRecursiveAnchor(root *ModelNode) (*PredicateTransformer, error) {
	stack := []*ModelNode{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if traceTransformerReaches(node.transformer, node.transformer, make(map[*PredicateTransformer]bool)) {
			return node.transformer, nil
		}
		children, err := node.Children()
		if err != nil {
			return nil, err
		}
		stack = append(stack, children...)
	}
	return nil, nil
}

func updateTraceModels(root *ModelNode) error {
	// The historical source keys this map only by state. Distinct nullary
	// predicates share the true cube, so copying its donor rule can attach a
	// different predicate's derivation. Match the search cache's predicate
	// identity as well as the projected state.
	byState := make(map[modelStateKey]*ModelNode)
	nodes := []*ModelNode{root}
	for index := 0; index < len(nodes); index++ {
		node := nodes[index]
		_, found, err := node.Model()
		if err != nil {
			return err
		}
		if found {
			byState[modelState(node)] = node
		}
		children, err := node.Children()
		if err != nil {
			return err
		}
		nodes = append(nodes, children...)
	}
	for _, node := range nodes {
		_, found, err := node.Model()
		if err != nil {
			return err
		}
		if found {
			continue
		}
		donor := byState[modelState(node)]
		if donor == nil {
			// As in source update_models, leave unmatched nodes untouched.
			// Constraint-cache initial hits have no expanded donor; their own
			// transformer selects an initial rule in selectedTraceRule.
			continue
		}
		modelCore, _, err := donor.Model()
		if err != nil {
			return err
		}
		if err = node.SetModel(modelCore); err != nil {
			return err
		}
		node.selected = donor.selected
	}
	return nil
}

// traceHasNonlinearRule identifies witnesses for which the retained native
// fixedpoint frontend reduces the printed trace to true. The linear recursive
// trace below must not try to recover models or select initial rules for every
// branch of such a witness.
func traceHasNonlinearRule(root *ModelNode) (bool, error) {
	stack := []*ModelNode{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if node.selected != nil {
			uninterpreted, err := node.selected.rule.UninterpretedTailSize()
			if err != nil {
				return false, err
			}
			if uninterpreted > 1 {
				return true, nil
			}
		}
		children, err := node.Children()
		if err != nil {
			return false, err
		}
		stack = append(stack, children...)
	}
	return false, nil
}

func (c *Context) unsatAnswer(queryDeclaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	// The direct expression path normally avoids constructing a model; source
	// to_expr constructs one and traces it before model-to-expression conversion.
	if c.manager.BorrowSourceTraceTags().ShouldTrace("pdr") {
		md, err := c.finiteFrameModel(c.inductive, queryDeclaration)
		if err != nil {
			return result, err
		}
		owner := &pdrModelExportOwner{manager: c.manager, model: md}
		if err = owner.dispose(nil); err != nil {
			return result, err
		}
	}
	references := make([]ast.ASTRef, 0)
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&references[index]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&result))
		}
	}()
	pin := func(handle ast.ASTHandle) error {
		reference, err := c.manager.NewRef(handle)
		if err == nil {
			references = append(references, reference)
		}
		return err
	}
	abstractor, err := ast.NewExprAbstractor(c.manager)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, abstractor.Close()) }()
	definitions := make([]ast.ASTHandle, 0, len(c.order))
	for _, declaration := range c.order {
		if declaration == queryDeclaration || c.auxiliary[declaration] {
			continue
		}
		transformer := c.transformers[declaration]
		bodyRef, x := transformer.FormulasRef(c.inductive, false)
		if x != nil {
			return result, x
		}
		references = append(references, bodyRef)
		body, x := c.manager.RefGet(bodyRef)
		if x != nil {
			return result, x
		}
		converted, x := c.owner.FormulaNewToOldRef(body, 0, false)
		if converted != (ast.ASTRef{}) {
			references = append(references, converted)
		}
		if x == nil {
			body, x = c.manager.RefGet(converted)
		}
		if x != nil {
			return result, x
		}
		fixed, x := c.fixupInductiveClausesRef(body)
		if fixed != (ast.ASTRef{}) {
			references = append(references, fixed)
		}
		if x != nil {
			return result, x
		}
		body, x = c.manager.RefGet(fixed)
		if x != nil {
			return result, x
		}
		arguments := make([]ast.ASTHandle, len(transformer.signature))
		for index, signature := range transformer.signature {
			arguments[index], x = c.manager.MkApp(signature, nil)
			if x != nil {
				return result, x
			}
			if x = pin(arguments[index]); x != nil {
				return result, x
			}
		}
		application, x := c.manager.MkApp(declaration, arguments)
		if x != nil {
			return result, x
		}
		if x = pin(application); x != nil {
			return result, x
		}
		equality, x := c.manager.MkEq(application, body)
		if x != nil {
			return result, x
		}
		if x = pin(equality); x != nil {
			return result, x
		}
		definition, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return result, x
		}
		references = append(references, definition)
		if x = abstractor.MkForall(arguments, equality, definition); x != nil {
			return result, x
		}
		definitionHandle, x := c.manager.RefGet(definition)
		if x != nil {
			return result, x
		}
		definitions = append(definitions, definitionHandle)
	}
	formula, err := c.manager.MkAndArray(definitions)
	if err != nil {
		return result, err
	}
	return c.manager.NewRef(formula)
}

// Model materializes the inductive relation interpretations for the latest
// unreachable query. The caller owns the returned model and closes it.
func (c *Context) Model(queryDeclaration ast.ASTHandle) (result *model.Model, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.lastResult != lbool.False || c.queryDecl != queryDeclaration {
		return nil, ast.ErrASTDomain
	}
	return c.finiteFrameModel(c.inductive, queryDeclaration)
}

// Answer reconstructs the selected satisfying rule trace or the direct
// predicate inductive certificate for the latest query. A successful unknown
// solve returns true, matching the source default branch. The caller releases
// the returned owning reference through ASTManager.DelRef.
func (c *Context) Answer(queryDeclaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if c.queryDecl != queryDeclaration {
		return result, ast.ErrASTDomain
	}
	if c.lastResult == lbool.Undef && c.unknownResult {
		truth, err := c.manager.MkTrue()
		if err != nil {
			return result, err
		}
		return c.manager.NewRef(truth)
	}
	if c.lastResult == lbool.False {
		return c.unsatAnswer(queryDeclaration)
	}
	if c.lastResult != lbool.True || c.query == nil || c.query.head != queryDeclaration {
		return result, ast.ErrASTDomain
	}
	if c.generateProofTrace {
		return c.Proof(queryDeclaration)
	}
	root, found, err := c.search.Root()
	if err != nil || !found {
		if err != nil {
			return result, err
		}
		return result, ast.ErrASTDomain
	}
	if !c.fullTraceAnswer {
		nonlinear, x := traceHasNonlinearRule(root)
		if x != nil {
			return result, fmt.Errorf("pdr: inspect trace shape: %w", x)
		}
		if nonlinear {
			truth, y := c.manager.MkTrue()
			if y != nil {
				return result, y
			}
			return c.manager.NewRef(truth)
		}
		anchor, x := traceRecursiveAnchor(root)
		if x != nil {
			return result, fmt.Errorf("pdr: inspect recursive trace: %w", x)
		}
		if anchor == nil {
			truth, y := c.manager.MkTrue()
			if y != nil {
				return result, y
			}
			return c.manager.NewRef(truth)
		}
	}
	constraints, err := c.unifiedTraceConstraints(root, c.fullTraceAnswer)
	if err != nil {
		return result, fmt.Errorf("pdr: unify trace: %w", err)
	}
	owner := &pdrAnswerExportOwner{manager: c.manager, temporaries: constraints}
	defer owner.finish(&result, &resultErr)
	handles := make([]ast.ASTHandle, len(constraints))
	for i := range constraints {
		handles[i], err = c.manager.RefGet(constraints[i])
		if err != nil {
			return result, fmt.Errorf("pdr: read answer constraint %d: %w", i, err)
		}
	}
	var formula ast.ASTHandle
	if len(handles) == 0 {
		formula, err = c.manager.MkTrue()
	} else {
		formula, err = c.manager.MkAndArgs(handles)
	}
	if err != nil {
		return result, fmt.Errorf("pdr: combine answer constraints: %w", err)
	}
	result, err = c.manager.NewRef(formula)
	if err != nil {
		return result, fmt.Errorf("pdr: retain answer formula: %w", err)
	}
	return result, nil
}
