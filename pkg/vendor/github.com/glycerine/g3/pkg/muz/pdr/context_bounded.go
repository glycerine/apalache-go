package pdr

// A narrow end-to-end Horn path assembled from the retained PDR transition
// encoding. Full unbounded solving remains in the model-search loop.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type boundedFormulaKey struct {
	transformer *PredicateTransformer
	depth       uint32
}

type boundedFormulaBuilder struct {
	context    *Context
	memo       map[boundedFormulaKey]ast.ASTHandle
	references []ast.ASTRef
}

func (b *boundedFormulaBuilder) pin(handle ast.ASTHandle) error {
	reference, err := b.context.manager.NewRef(handle)
	if err == nil {
		b.references = append(b.references, reference)
	}
	return err
}

func (b *boundedFormulaBuilder) formula(transformer *PredicateTransformer, depth uint32) (ast.ASTHandle, error) {
	key := boundedFormulaKey{transformer: transformer, depth: depth}
	if prior, found := b.memo[key]; found {
		return prior, nil
	}
	if depth == 0 {
		initial, err := transformer.InitialState()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		b.memo[key] = initial
		return initial, nil
	}
	transition, err := transformer.Transition()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction := []ast.ASTHandle{transition}
	for _, rule := range transformer.rules {
		if rule.tag.IsNull() {
			continue
		}
		for index, predecessor := range rule.predecessors {
			child, x := b.formula(predecessor, depth-1)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			child, x = b.context.owner.FormulaNewToOld(child, uint32(index), false)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			guarded, x := b.context.manager.MkImplies(rule.tag, child)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if x = b.pin(guarded); x != nil {
				return ast.ASTHandle{}, x
			}
			conjunction = append(conjunction, guarded)
		}
	}
	result, err := b.context.manager.MkAndArray(conjunction)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	b.memo[key] = result
	return result, nil
}

func (b *boundedFormulaBuilder) close() (result error) {
	for index := len(b.references) - 1; index >= 0; index-- {
		result = errors.Join(result, b.context.manager.DelRef(&b.references[index]))
	}
	b.references = nil
	b.memo = nil
	return result
}

// CheckBounded tests whether a concrete predicate application is derivable in
// at most depth rule steps. It recursively composes tag-selected transitions
// with predecessor reachability formulas and discharges the result through
// the common SMT backend.
func (c *Context) CheckBounded(query ast.ASTHandle, depth uint32, wantModel bool) (result PropCheckResult, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if !c.initialized {
		return result, ast.ErrASTDomain
	}
	transformer, state, queryReferences, err := c.queryState(query)
	if err != nil {
		return result, err
	}
	defer func() {
		for index := len(queryReferences) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&queryReferences[index]))
		}
	}()
	builder := &boundedFormulaBuilder{context: c, memo: make(map[boundedFormulaKey]ast.ASTHandle)}
	defer func() { resultErr = errors.Join(resultErr, builder.close()) }()
	formula, err := builder.formula(transformer, depth)
	if err != nil {
		return result, err
	}
	solver, err := c.owner.NewPropSolver("#bounded")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, solver.Close()) }()
	if err = solver.AddFormula(formula); err != nil {
		return result, err
	}
	return solver.CheckAssumptions([]ast.ASTHandle{state}, false, wantModel)
}
