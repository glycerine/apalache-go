package fp

// Source: spacer_dl_interface.cpp saves old_rules and performs query
// preprocessing on a private rule-set copy.

import "errors"

func (e *PropositionalEngine) copyQueryRules(clone *PropositionalEngine) (resultErr error) {
	formulas, names, err := e.NamedRulesWithSymbols()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, formulas)) }()
	for i := range formulas {
		formula, x := e.manager.RefGet(formulas[i])
		if x != nil {
			return x
		}
		if x = clone.AddRuleWithSymbol(formula, names[i]); x != nil {
			return x
		}
	}
	return nil
}
