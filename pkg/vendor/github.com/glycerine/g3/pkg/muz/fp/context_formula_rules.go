package fp

// Source: src/muz/base/dl_context.cpp get_rules_as_formulas and
// src/api/api_datalog.cpp Z3_fixedpoint_get_rules.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// contextFormulaRules returns a borrowed persistent set or a caller-owned
// snapshot excluding pending closed inputs. Pending handles remain borrowed.
func (e *PropositionalEngine) contextFormulaRules() (_ *base.RuleSet, pending []ast.ASTHandle, names []symbol.Symbol, resultErr error) {
	if err := e.normalizePendingOpenRules(); err != nil {
		return nil, nil, nil, err
	}
	count, err := e.set.Len()
	if err != nil {
		return nil, nil, nil, err
	}
	omit := make([]bool, count)
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		h, err := e.manager.RefGet(raw.formula)
		if err != nil {
			return nil, nil, nil, err
		}
		if raw.first < 0 || raw.end > count || raw.first > raw.end {
			return nil, nil, nil, ErrPropositionalEngine
		}
		for i := raw.first; i < raw.end; i++ {
			omit[i] = true
		}
		pending = append(pending, h)
		names = append(names, raw.name)
	}
	if len(pending) == 0 {
		return e.set, pending, names, nil
	}
	snapshot, err := base.NewRuleSet(e.rules)
	if err != nil {
		return nil, nil, nil, err
	}
	a := &contextFormulaAttempt{manager: e.manager, sets: []*base.RuleSet{snapshot}}
	defer func() { resultErr = a.finish(resultErr) }()
	if err = snapshot.InheritPredicates(e.set); err != nil {
		return nil, nil, nil, err
	}
	for i := 0; i < count; i++ {
		if omit[i] {
			continue
		}
		rule, err := e.set.Rule(i)
		if rule != nil {
			a.rules = append(a.rules, rule)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		err = snapshot.Add(rule)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if err = a.closeRules(); err != nil {
		return nil, nil, nil, err
	}
	a.sets = nil
	return snapshot, pending, names, nil
}

// RuleFormulas returns the source public rule enumeration: normalized clauses,
// pending closed inputs, then negated output queries. The caller owns each ref.
func (e *PropositionalEngine) RuleFormulas() (result []ast.ASTRef, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	rules, pending, _, err := e.contextFormulaRules()
	if err != nil {
		return nil, err
	}
	var queries []ast.ASTRef
	a := &contextFormulaAttempt{manager: e.manager}
	if rules != e.set {
		a.sets = append(a.sets, rules)
	}
	defer func() {
		a.references = append(a.references, queries...)
		if resultErr != nil {
			a.references = append(a.references, result...)
			result = nil
		}
		resultErr = a.finish(resultErr)
		if resultErr != nil && result != nil {
			// Failed temporary cleanup prevents publication. The returned
			// receipt still owns the attempt and these unpublished outputs.
			a.references = append(a.references, result...)
			result = nil
		}
	}()
	var names []symbol.Symbol
	if err = rules.AppendDisplayFormulasWithSymbols(&result, &queries, &names); err != nil {
		return result, err
	}
	for _, formula := range pending {
		ref, err := e.manager.NewRef(formula)
		if err != nil {
			return result, err
		}
		result = append(result, ref)
	}
	for i, query := range queries {
		h, err := e.manager.RefGet(query)
		if err != nil {
			return result, err
		}
		negated, err := e.manager.MkNot(h)
		if err != nil {
			return result, err
		}
		if err = e.manager.SetRef(query, negated); err != nil {
			return result, err
		}
		result = append(result, query)
		queries[i] = ast.ASTRef{}
	}
	return result, nil
}
