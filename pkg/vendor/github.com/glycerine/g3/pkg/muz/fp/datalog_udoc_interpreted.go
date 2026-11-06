package fp

// Source: dl_compiler.cpp compile_rule translates rule variables to relation
// columns, fusing interpreted filtering with projection of variables absent
// from the head before invoking the udoc plugin.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocGuardAttempt struct {
	manager      *ast.ASTManager
	roots        []ast.ASTRef
	substitution *astrewriter.VarSubstitution
	mapped       ast.ASTRef
}

func (a *udocGuardAttempt) close() error {
	if a.mapped != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.mapped); err != nil {
			return err
		}
	}
	if a.substitution != nil {
		if err := a.substitution.Close(); err != nil {
			return err
		}
		a.substitution = nil
	}
	for len(a.roots) > 0 {
		index := len(a.roots) - 1
		if a.roots[index] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.roots[index]); err != nil {
				return err
			}
		}
		a.roots = a.roots[:index]
	}
	return nil
}

func (c *datalogUDocContext) clearGuardCleanup() error {
	for len(c.guardCleanup) > 0 {
		index := len(c.guardCleanup) - 1
		if err := c.guardCleanup[index].close(); err != nil {
			return err
		}
		c.guardCleanup = c.guardCleanup[:index]
	}
	return nil
}

func (c *datalogUDocContext) filterConditionAndProject(working *rel.TypedUDocRelation, conditions []ast.ASTHandle, variables map[uint32]uint32, removed []uint32) (result *rel.TypedUDocRelation, resultErr error) {
	attempt := &udocGuardAttempt{manager: c.manager}
	c.guardCleanup = append(c.guardCleanup, attempt)
	defer func() {
		resultErr = errors.Join(resultErr, c.clearGuardCleanup())
		if resultErr != nil && result != nil {
			if result != working {
				resultErr = errors.Join(resultErr, result.Close())
			}
			result = nil
		}
	}()
	condition, err := c.manager.MkAndArgs(conditions)
	if err != nil {
		return nil, err
	}
	conditionRoot, err := c.manager.NewRef(condition)
	if err != nil {
		return nil, err
	}
	attempt.roots = append(attempt.roots, conditionRoot)
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, err
	}
	sorts := working.Sorts()
	bindings := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		column, found := variables[variable]
		if !found || int(column) >= len(sorts) || sort != sorts[column] {
			return nil, fmt.Errorf("%w: unbound interpreted variable", ErrUDocDatalogFragment)
		}
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		value, cause := c.manager.MkVar(variables[variable], sort)
		if cause != nil {
			return nil, cause
		}
		root, cause := c.manager.NewRef(value)
		if cause != nil {
			return nil, cause
		}
		attempt.roots = append(attempt.roots, root)
		bindings[variable] = value
	}
	attempt.substitution, err = astrewriter.NewVarSubstitution(c.manager, false)
	if err != nil {
		return nil, err
	}
	attempt.mapped, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if err = attempt.substitution.Substitute(condition, bindings, attempt.mapped); err != nil {
		return nil, err
	}
	mapped, err := c.manager.RefGet(attempt.mapped)
	if err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		if err = working.FilterInterpreted(mapped); err != nil {
			return nil, errors.Join(err, ErrUDocDatalogFragment)
		}
		return working, nil
	}
	result, err = working.FilterInterpretedAndProject(mapped, removed)
	if err != nil {
		return result, errors.Join(err, ErrUDocDatalogFragment)
	}
	return result, nil
}

func (c *datalogUDocContext) filterInterpretedTailsAndProject(working *rel.TypedUDocRelation, rule *base.Rule, head ast.ASTHandle, first, total int, variables, columnVariables map[uint32]uint32) (*rel.TypedUDocRelation, error) {
	if first == total {
		return working, nil
	}
	conditions := make([]ast.ASTHandle, 0, total-first)
	for i := first; i < total; i++ {
		if err := c.checkpoint(); err != nil {
			return nil, err
		}
		condition, negated, err := rule.Tail(i)
		if err != nil {
			return nil, err
		}
		if negated {
			return nil, ErrUDocDatalogFragment
		}
		conditions = append(conditions, condition)
	}
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Collect(head); err != nil {
		return nil, err
	}
	var removed []uint32
	for column, variable := range columnVariables {
		if used.Contains(variable).IsNull() {
			removed = append(removed, column)
		}
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	result, err := c.filterConditionAndProject(working, conditions, variables, removed)
	if err != nil {
		return nil, err
	}
	for variable, column := range variables {
		if used.Contains(variable).IsNull() {
			delete(variables, variable)
			continue
		}
		var shift uint32
		for _, removedColumn := range removed {
			if removedColumn < column {
				shift++
			}
		}
		variables[variable] = column - shift
	}
	return result, nil
}
