package pdr

// Source: the retained reduce_disequalities path in
// src/muz/pdr/pdr_util.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (c *Context) disequalitySides(expression ast.ASTHandle) (left, right ast.ASTHandle, matched bool, result error) {
	negated, equality, err := c.manager.IsNot(expression)
	if err != nil || !negated {
		return left, right, false, err
	}
	kind, err := c.manager.GetKind(equality)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := c.manager.GetFamilyID(equality)
	if err != nil || family != c.manager.BasicFamilyID() {
		return left, right, false, err
	}
	operation, err := c.manager.GetDeclKind(equality)
	if err != nil || operation != ast.ASTOpEq {
		return left, right, false, err
	}
	left, err = c.manager.GetArg(equality, 0)
	if err != nil {
		return left, right, false, err
	}
	right, err = c.manager.GetArg(equality, 1)
	return left, right, err == nil, err
}

// reduceDisequalities replaces a term excluded from at least threshold values
// by its model value, retaining the defining equality. The returned reference
// is owning.
func (c *Context) reduceDisequalities(md *model.Model, threshold uint32, formula ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	if c.implicant == nil || md == nil || threshold == 0 {
		return c.manager.NewRef(formula)
	}
	conjuncts, err := c.manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return result, err
	}
	counts := make(map[ast.ASTHandle]uint32)
	order := make([]ast.ASTHandle, 0)
	for _, conjunct := range conjuncts {
		left, right, matched, x := c.disequalitySides(conjunct)
		if x != nil {
			return result, x
		}
		if !matched {
			continue
		}
		value, x := c.manager.IsValue(right)
		if x != nil {
			return result, x
		}
		if !value {
			left, right = right, left
			value, x = c.manager.IsValue(right)
			if x != nil {
				return result, x
			}
		}
		if value {
			if counts[left] == 0 {
				order = append(order, left)
			}
			counts[left]++
		}
	}
	substitution, err := c.manager.NewExprSubstitutionPolicy(false, false)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelExprSubstitution(&substitution)) }()
	references := make([]ast.ASTRef, 0)
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(c.manager, references)) }()
	definitions := make([]ast.ASTHandle, 0)
	for _, term := range order {
		count := counts[term]
		if count < threshold {
			continue
		}
		value, x := c.implicant.EvalExpr(md, term)
		if x != nil {
			return result, x
		}
		references = append(references, value)
		valueHandle, x := c.manager.RefGet(value)
		if x != nil {
			return result, x
		}
		if x = c.manager.ExprSubstitutionInsert(substitution, term, valueHandle, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
			return result, x
		}
		definition, x := c.manager.MkEq(term, valueHandle)
		if x != nil {
			return result, x
		}
		definitionRef, x := c.manager.NewRef(definition)
		if x != nil {
			return result, x
		}
		references = append(references, definitionRef)
		definitions = append(definitions, definition)
	}
	if len(definitions) == 0 {
		return c.manager.NewRef(formula)
	}
	replacer, err := rewriter.NewSimplifyingExprReplacer(c.manager, c.owner.contexts.config.Params, c.owner.contexts.config.Globals, c.owner.contexts.params)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
	if err = replacer.SetSubstitution(&substitution); err != nil {
		return result, err
	}
	reduced := make([]ast.ASTHandle, 0, len(conjuncts)+len(definitions))
	for _, conjunct := range conjuncts {
		rewritten, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return result, x
		}
		references = append(references, rewritten)
		if x = replacer.Rewrite(conjunct, rewritten); x != nil {
			return result, x
		}
		handle, x := c.manager.RefGet(rewritten)
		if x != nil {
			return result, x
		}
		if !c.manager.IsTrue(handle) {
			reduced = append(reduced, handle)
		}
	}
	reduced = append(reduced, definitions...)
	formula, err = c.manager.MkAndArray(reduced)
	if err != nil {
		return result, err
	}
	return c.manager.NewRef(formula)
}
