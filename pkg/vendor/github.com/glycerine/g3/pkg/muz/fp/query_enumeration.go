package fp

// Source: dl_context rule/background vectors copied by fixedpoint query setup.
// Keep construction roots separate from named return values during unwinding.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// BackgroundAssertions returns owning references in insertion order.
func (e *PropositionalEngine) BackgroundAssertions() (result []ast.ASTRef, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	owned := make([]ast.ASTRef, 0, len(e.assertions))
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(e.manager, owned))
			result = nil
		}
	}()
	for _, assertion := range e.assertions {
		handle, err := e.manager.RefGet(assertion)
		if err != nil {
			return nil, err
		}
		reference, err := e.manager.NewRef(handle)
		if err != nil {
			return nil, err
		}
		owned = append(owned, reference)
	}
	return owned, nil
}

// NamedRules returns owning formulas and their copied names in insertion
// order.
func (e *PropositionalEngine) NamedRules() (result []ast.ASTRef, names []string, resultErr error) {
	formulas, labels, err := e.NamedRulesWithSymbols()
	if err != nil {
		return nil, nil, err
	}
	names, err = e.ruleNameStrings(labels)
	if err != nil {
		a := &contextFormulaAttempt{manager: e.manager, references: formulas}
		return nil, nil, a.finish(err)
	}
	return formulas, names, nil
}

// NamedRulesWithSymbols returns owning formulas and exact source label
// identities. Each returned formula belongs to the caller.
func (e *PropositionalEngine) NamedRulesWithSymbols() (result []ast.ASTRef, names []symbol.Symbol, resultErr error) {
	if err := e.open(); err != nil {
		return nil, nil, err
	}
	count, err := e.set.Len()
	if err != nil {
		return nil, nil, err
	}
	owned := make([]ast.ASTRef, 0, count)
	a := &contextFormulaAttempt{manager: e.manager}
	defer func() {
		if resultErr != nil {
			a.references = append(a.references, owned...)
			result = nil
			names = nil
		}
		resultErr = a.finish(resultErr)
		if resultErr != nil && result != nil {
			a.references = append(a.references, result...)
			result = nil
			names = nil
		}
	}()
	names = make([]symbol.Symbol, 0, count)
	for index := 0; index < count; index++ {
		rule, getErr := e.set.Rule(index)
		if rule != nil {
			a.rules = append(a.rules, rule)
		}
		if getErr != nil {
			return nil, nil, getErr
		}
		formula, formulaErr := rule.ToFormula()
		name, nameErr := rule.NameSymbol()
		formulaErr = errors.Join(formulaErr, nameErr)
		if formula != (ast.ASTRef{}) {
			owned = append(owned, formula)
		}
		if formulaErr != nil {
			return nil, nil, formulaErr
		}
		names = append(names, name)
	}
	return owned, names, nil
}
