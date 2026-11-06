package rel

// Source: finite_product_relation_plugin::filter_interpreted_fn remaps
// relation columns to its finite table or inner explanation signature.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (i *interpretedFilterInstruction) explanationConditionVariables(product *FiniteExplanationProduct, condition ast.ASTHandle) (finite, explanation map[uint32]ast.ASTHandle, resultErr error) {
	used, err := ast.NewASTUsedVars(i.manager)
	if err != nil {
		return nil, nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, nil, err
	}
	finite = make(map[uint32]ast.ASTHandle)
	explanation = make(map[uint32]ast.ASTHandle)
	var found uint32
	for variable, sort := range i.variableSorts {
		actual := used.Contains(variable)
		if actual.IsNull() {
			continue
		}
		if actual != sort {
			return nil, nil, ErrInstruction
		}
		column, ok := i.variableColumns[variable]
		if !ok || column < 0 || column >= len(product.finiteSorts)+product.explanationColumns {
			return nil, nil, ErrInstruction
		}
		rule, err := i.manager.IsDatalogRuleSort(sort)
		if err != nil {
			return nil, nil, err
		}
		if rule {
			if column < len(product.finiteSorts) {
				return nil, nil, ErrInstruction
			}
			explanation[variable] = sort
		} else {
			if column >= len(product.finiteSorts) || product.finiteSorts[column] != sort {
				return nil, nil, ErrInstruction
			}
			finite[variable] = sort
		}
		found++
	}
	if found != used.NumVars() {
		return nil, nil, ErrInstruction
	}
	return finite, explanation, nil
}

func (i *interpretedFilterInstruction) assignExplanationCondition(product *FiniteExplanationProduct, condition ast.ASTHandle, variables map[uint32]ast.ASTHandle) (resultErr error) {
	var maximum uint32
	for variable := range variables {
		if variable > maximum {
			maximum = variable
		}
	}
	bindings := make([]ast.ASTHandle, uint64(maximum)+1)
	var refs []ast.ASTRef
	defer func() {
		for index := len(refs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, i.manager.DelRef(&refs[index]))
		}
	}()
	for index := range bindings {
		variable := uint32(index)
		sort, found := variables[variable]
		if !found {
			continue
		}
		column := i.variableColumns[variable] - len(product.finiteSorts)
		value, err := i.manager.MkVar(uint32(column), sort)
		if err != nil {
			return err
		}
		ref, err := i.manager.NewRef(value)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		bindings[variable] = value
	}
	sub, err := rewriter.NewVarSubstitution(i.manager, false)
	if sub != nil {
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := i.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	refs = append(refs, out)
	if err = sub.Substitute(condition, bindings, out); err != nil {
		return err
	}
	mapped, err := i.manager.RefGet(out)
	if err != nil {
		return err
	}
	return product.AssignExplanation(mapped)
}

func (i *interpretedFilterInstruction) performExplanationFilter(context *ExecutionContext) (bool, error) {
	product, err := context.ExplanationRegister(i.register)
	if err != nil || product == nil {
		return err == nil, err
	}
	if product.manager != i.relations {
		return false, ErrInstruction
	}
	context.stats.Filter++
	for _, reference := range i.conditions {
		condition, err := i.manager.RefGet(reference)
		if err != nil {
			return false, err
		}
		finite, explanation, err := i.explanationConditionVariables(product, condition)
		if err != nil {
			return false, err
		}
		if len(explanation) != 0 {
			// Source-generated explanation records contain only rule-sort variables.
			// Mixed arithmetic/rule records still need the row-specialization path.
			if len(finite) != 0 {
				return false, ErrInstruction
			}
			if err = i.assignExplanationCondition(product, condition, explanation); err != nil {
				return false, err
			}
			continue
		}
		// The existing finite evaluator receives only this condition's variables;
		// rule-sort columns are never decoded as bounded table elements.
		finiteFilter := *i
		finiteFilter.conditions = []ast.ASTRef{reference}
		finiteFilter.variableSorts = finite
		rows, err := product.table.Facts()
		if err != nil {
			return false, err
		}
		for _, row := range rows {
			accepted, err := finiteFilter.accepts(row[:len(row)-1])
			if err != nil {
				return false, err
			}
			if !accepted {
				if _, err = product.table.Remove(row); err != nil {
					return false, err
				}
			}
		}
	}
	count, err := product.Len()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, context.MakeEmpty(i.register)
	}
	return true, nil
}
