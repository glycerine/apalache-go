package base

// Source: src/muz/base/dl_rule.cpp rule_manager::reduce_unbound_vars and
// rule_manager::fix_unbound_vars.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func closeASTRefs(manager *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for index := len(refs) - 1; index >= 0; index-- {
		result = errors.Join(result, manager.DelRef(&refs[index]))
	}
	return result
}

func (m *RuleManager) checkRuleOwner(rule *Rule) error {
	if err := m.open(); err != nil {
		return err
	}
	owner, err := rule.ASTManager()
	if err != nil {
		return err
	}
	if owner != m.manager {
		return fmt.Errorf("muz/base: rule belongs to a different AST manager")
	}
	return nil
}

func collectRuleBoundVariables(manager *ast.ASTManager, rule *Rule, uninterpreted int) (*ast.ASTUsedVars, error) {
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	if err = used.Collect(head); err != nil {
		return nil, err
	}
	for index := 0; index < uninterpreted; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		if err = used.Process(tail); err != nil {
			return nil, err
		}
	}
	return used, nil
}

func usedVariableIndices(used *ast.ASTUsedVars) []uint32 {
	indices := make([]uint32, 0, used.NumVars())
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		if used.Contains(index) != (ast.ASTHandle{}) {
			indices = append(indices, index)
		}
	}
	return indices
}

func (m *RuleManager) rebuildRule(rule *Rule, interpreted []ast.ASTHandle) (*Rule, error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, uninterpreted+len(interpreted))
	negated := make([]bool, 0, cap(tails))
	for index := 0; index < uninterpreted; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		tails = append(tails, tail)
		negated = append(negated, negative)
	}
	var temporaries []ast.ASTRef
	defer func() { _ = closeASTRefs(m.manager, temporaries) }()
	for _, expression := range interpreted {
		application, ensureErr := m.ensureBooleanApplication(expression, &temporaries)
		if ensureErr != nil {
			return nil, ensureErr
		}
		tails = append(tails, application)
		negated = append(negated, false)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, err
	}
	return m.NewRuleWithSymbol(head, tails, negated, name)
}

// ReduceUnboundVariables applies qe_lite to interpreted tails, selecting the
// variables absent from the rule head and predicate tails. The returned rule
// is always an owner; changed reports whether it is a rebuilt rule.
func (m *RuleManager) ReduceUnboundVariables(rule *Rule, lite *qe.Lite) (_ *Rule, changed bool, resultErr error) {
	if err := m.checkRuleOwner(rule); err != nil {
		return nil, false, err
	}
	if lite == nil {
		return nil, false, fmt.Errorf("muz/base: nil unbound-variable eliminator")
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	if uninterpreted == total {
		retained, retainErr := rule.Retain()
		return retained, false, retainErr
	}
	bound, err := collectRuleBoundVariables(m.manager, rule, uninterpreted)
	if err != nil {
		return nil, false, err
	}
	forms := make([]ast.ASTRef, 0, total-uninterpreted)
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(m.manager, forms)) }()
	before := make([]ast.ASTHandle, 0, total-uninterpreted)
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		ref, getErr := m.manager.NewRef(tail)
		if getErr != nil {
			return nil, false, getErr
		}
		forms = append(forms, ref)
		before = append(before, tail)
	}
	if err = lite.EliminateConjunctions(usedVariableIndices(bound), false, &forms); err != nil {
		return nil, false, fmt.Errorf("muz/base: reduce unbound variables: %w", err)
	}
	changed = len(forms) != len(before)
	interpreted := make([]ast.ASTHandle, len(forms))
	for index, ref := range forms {
		interpreted[index], err = m.manager.RefGet(ref)
		if err != nil {
			return nil, false, err
		}
		changed = changed || interpreted[index] != before[index]
	}
	if !changed {
		retained, retainErr := rule.Retain()
		return retained, false, retainErr
	}
	rebuilt, err := m.rebuildRule(rule, interpreted)
	if err != nil {
		return nil, false, fmt.Errorf("muz/base: rebuild reduced rule: %w", err)
	}
	if err = m.MkRuleRewriteProof(rule, rebuilt); err != nil {
		return nil, false, errors.Join(err, rebuilt.Close())
	}
	return rebuilt, true, nil
}

// FixUnboundVariables first performs source QE-lite reduction. When enabled,
// remaining interpreted conjuncts containing variables absent from predicate
// positions are combined under an existential quantifier.
func (m *RuleManager) FixUnboundVariables(rule *Rule, lite *qe.Lite, enabled bool) (_ *Rule, changed bool, resultErr error) {
	current, reduced, err := m.ReduceUnboundVariables(rule, lite)
	if err != nil {
		return nil, false, err
	}
	keep := false
	defer func() {
		if !keep || resultErr != nil {
			resultErr = errors.Join(resultErr, current.Close())
		}
	}()
	if !enabled {
		keep = true
		return current, reduced, nil
	}
	uninterpreted, err := current.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	total, err := current.TailSize()
	if err != nil || uninterpreted == total {
		if err == nil {
			keep = true
		}
		return current, reduced, err
	}
	bound, err := collectRuleBoundVariables(m.manager, current, uninterpreted)
	if err != nil {
		return nil, false, err
	}
	all, err := ast.NewASTUsedVars(m.manager)
	if err != nil {
		return nil, false, err
	}
	head, err := current.Head()
	if err != nil {
		return nil, false, err
	}
	if err = all.Collect(head); err != nil {
		return nil, false, err
	}
	kept := make([]ast.ASTHandle, 0, total-uninterpreted)
	var grouped []ast.ASTHandle
	unbound := make(map[uint32]bool)
	for index := 0; index < total; index++ {
		tail, _, getErr := current.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		if err = all.Process(tail); err != nil {
			return nil, false, err
		}
		if index < uninterpreted {
			continue
		}
		free, getErr := ast.NewASTUsedVars(m.manager)
		if getErr != nil {
			return nil, false, getErr
		}
		if getErr = free.Collect(tail); getErr != nil {
			return nil, false, getErr
		}
		hasUnbound := false
		for variable := uint32(0); variable < free.MaxFoundVarIndexPlusOne(); variable++ {
			if free.Contains(variable) != (ast.ASTHandle{}) && bound.Contains(variable) == (ast.ASTHandle{}) {
				unbound[variable] = true
				hasUnbound = true
			}
		}
		if hasUnbound {
			grouped = append(grouped, tail)
		} else {
			kept = append(kept, tail)
		}
	}
	if len(grouped) == 0 {
		keep = true
		return current, reduced, nil
	}
	// The source unbound_tail expr_ref owns the conjunction before any
	// substitution bindings or rewriter state are allocated.
	bodyRef, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&bodyRef)) }()
	body, err := m.manager.MkAndArray(grouped)
	if err = errors.Join(err, m.manager.SetRef(bodyRef, body)); err != nil {
		return nil, false, err
	}
	qcount := uint32(len(unbound))
	arguments := make([]ast.ASTHandle, all.MaxFoundVarIndexPlusOne())
	// Source keeps the replacement variables in an expr_ref_vector through
	// substitution, including when constructing the rewriter fails.
	var argumentRefs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(m.manager, argumentRefs)) }()
	quantifierSorts := make([]ast.ASTHandle, 0, qcount)
	qindex := uint32(0)
	for variable := uint32(0); variable < all.MaxFoundVarIndexPlusOne(); variable++ {
		sort := all.Contains(variable)
		if sort == (ast.ASTHandle{}) {
			continue
		}
		newIndex := variable + qcount
		if unbound[variable] {
			newIndex = qindex
			qindex++
			quantifierSorts = append(quantifierSorts, sort)
		}
		owner, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return nil, false, e
		}
		argumentRefs = append(argumentRefs, owner)
		arguments[variable], err = m.manager.MkVar(newIndex, sort)
		if err = errors.Join(err, m.manager.SetRef(owner, arguments[variable])); err != nil {
			return nil, false, err
		}
	}
	substitution, err := rewriter.NewVarSubstitution(m.manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return nil, false, err
	}
	if err = substitution.Substitute(body, arguments, bodyRef); err != nil {
		return nil, false, err
	}
	body, err = m.manager.RefGet(bodyRef)
	if err != nil {
		return nil, false, err
	}
	for left, right := 0, len(quantifierSorts)-1; left < right; left, right = left+1, right-1 {
		quantifierSorts[left], quantifierSorts[right] = quantifierSorts[right], quantifierSorts[left]
	}
	names := make([]usymbol.Symbol, len(quantifierSorts))
	for index := range names {
		names[len(names)-1-index] = usymbol.Numerical(uint32(index))
	}
	fixedRef, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&fixedRef)) }()
	fixed, err := m.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: quantifierSorts, Names: names, Body: body})
	if err = errors.Join(err, m.manager.SetRef(fixedRef, fixed)); err != nil {
		return nil, false, err
	}
	simplified, err := qe.SimplifyQuantifiers(m.manager, fixed)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&simplified)) }()
	fixed, err = m.manager.RefGet(simplified)
	if err != nil {
		return nil, false, err
	}
	if !m.manager.IsTrue(fixed) {
		kept = append(kept, fixed)
	}
	rebuilt, err := m.rebuildRule(current, kept)
	if err != nil {
		return nil, false, fmt.Errorf("muz/base: rebuild fixed rule: %w", err)
	}
	if err = m.MkRuleRewriteProof(current, rebuilt); err != nil {
		return nil, false, errors.Join(err, rebuilt.Close())
	}
	if err = rebuilt.SetAccountingParentRule(current); err != nil {
		return nil, false, errors.Join(err, rebuilt.Close())
	}
	if err = current.Close(); err != nil {
		return nil, false, errors.Join(err, rebuilt.Close())
	}
	current = rebuilt
	keep = true
	return current, true, nil
}
