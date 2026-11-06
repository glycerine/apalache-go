package fp

// Source: dl_context.cpp context::get_rules_as_formulas, pending free-variable
// normalization and swap removal. The eager Go clauses for pending inputs
// are replaced by fresh normalization using the current context parameters.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) normalizePendingOpenRules() error {
	return e.normalizePendingRules(false)
}

func (e *PropositionalEngine) normalizePendingRules(flush bool) (resultErr error) {
	if !flush {
		return e.normalizePendingOpenSourceRules()
	}
	return e.flushPendingSourceRules()
}

// removePendingEagerRules first removes the eager Go-only copies of pending
// formulas. Source has not inserted those copies, so subsequent mk_rule calls
// must append directly to the persistent set, including on partial failure.
func (e *PropositionalEngine) removePendingEagerRules() (prefix int, resultErr error) {
	count, err := e.set.Len()
	if err != nil {
		return 0, err
	}
	pending := make([]bool, count)
	hasPending := false
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		hasPending = true
		if raw.first < 0 || raw.first > raw.end || raw.end > count {
			return 0, ErrPropositionalEngine
		}
		for i := raw.first; i < raw.end; i++ {
			pending[i] = true
		}
	}
	if !hasPending {
		return 0, nil
	}
	replacement, err := base.NewRuleSet(e.rules)
	if err != nil {
		return 0, err
	}
	attempt := &contextFormulaAttempt{manager: e.manager, sets: []*base.RuleSet{replacement}}
	defer func() { resultErr = attempt.finish(resultErr) }()
	for i := 0; i < count; i++ {
		if pending[i] {
			continue
		}
		rule, err := e.set.Rule(i)
		if rule != nil {
			attempt.rules = append(attempt.rules, rule)
		}
		if err != nil {
			return 0, err
		}
		if err = replacement.Add(rule); err != nil {
			return 0, err
		}
	}
	if err = replacement.InheritPredicates(e.set); err != nil {
		return 0, err
	}
	if err = e.rebindArithmeticRules(e.set); err != nil {
		return 0, err
	}
	if err = attempt.closeRules(); err != nil {
		return 0, err
	}
	prefix, err = replacement.Len()
	if err != nil {
		return 0, err
	}
	if err = e.set.ExchangeContents(replacement); err != nil {
		return 0, err
	}
	for i := range e.rawRules {
		if e.rawRules[i].pending {
			e.rawRules[i].first = prefix
			e.rawRules[i].end = prefix
		}
	}
	// Release the removed eager copies before entering source rule creation;
	// after this checkpoint the persistent set owns every published clause.
	if err = attempt.close(); err != nil {
		return 0, err
	}
	return prefix, nil
}

func (e *PropositionalEngine) flushPendingSourceRules() (resultErr error) {
	prefix, err := e.removePendingEagerRules()
	if err != nil {
		return err
	}

	for {
		index := -1
		for i, raw := range e.rawRules {
			if raw.pending {
				index = i
				break
			}
		}
		if index < 0 {
			return nil
		}
		err := e.flushPendingSourceRule(e.rawRules[index])
		length, lenErr := e.set.Len()
		if lenErr != nil {
			return errors.Join(err, lenErr)
		}
		if length != prefix || err == nil {
			e.advanceRuleRevision()
			prefix = length
		}
		if err != nil {
			return err
		}
		e.rawRules[index].pending = false
	}
}

// flushPendingSourceRule owns the bound formula even when conversion or cleanup
// fails, while any rules already added to e.set remain published.
func (e *PropositionalEngine) flushPendingSourceRule(raw rawRuleFormula) (resultErr error) {
	proofOwner := &pendingFlushProofOwner{manager: e.manager}
	defer func() { resultErr = proofOwner.finish(resultErr) }()
	a := &contextFormulaAttempt{manager: e.manager}
	defer func() { resultErr = a.finish(resultErr) }()
	formula, err := e.manager.RefGet(raw.formula)
	if err != nil {
		return err
	}
	enabled, err := e.pendingProofEnabled()
	if err != nil {
		return err
	}
	var proof ast.ASTHandle
	if enabled {
		proofOwner.proof, err = e.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		proof, err = e.manager.MkProofAsserted(formula)
		if !proof.IsNull() {
			err = errors.Join(err, e.manager.SetRef(proofOwner.proof, proof))
		}
		if err != nil {
			return err
		}
	}
	// mk_rule enters its own proof mode after flush_add_rules creates the
	// original assertion, then replaces that assertion if binding changes it.
	mode := ast.ASTProofDisabled
	if enabled {
		mode = ast.ASTProofEnabled
	}
	proofOwner.scope, err = ast.NewScopedProofMode(e.manager, mode)
	if err != nil {
		return err
	}
	original := formula
	bound, err := e.binder.Bind(formula, true)
	if bound != (ast.ASTRef{}) {
		a.references = append(a.references, bound)
	}
	if err != nil {
		return err
	}
	formula, err = e.manager.RefGet(bound)
	if err != nil {
		return err
	}
	if enabled && formula != original {
		proof, err = e.manager.MkProofAsserted(formula)
		if !proof.IsNull() {
			err = errors.Join(err, e.manager.SetRef(proofOwner.proof, proof))
		}
		if err != nil {
			return err
		}
	}
	return e.addFormulaWithProof(e.set, formula, proof, raw.name)
}

// normalizePendingOpenSourceRules mirrors get_rules_as_formulas: each open
// formula writes directly to the persistent set and is removed only after
// mk_rule returns. A failure may leave already-created clauses in that set.
func (e *PropositionalEngine) normalizePendingOpenSourceRules() (resultErr error) {
	freeVars, err := rewriter.NewExprFreeVars(e.manager)
	if err != nil {
		return err
	}
	hasFreeVars := func(formula ast.ASTHandle) (bool, error) {
		if err := freeVars.Collect(formula); err != nil {
			return false, err
		}
		return !freeVars.Empty(), nil
	}
	firstOpen := -1
	for i, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		formula, err := e.manager.RefGet(raw.formula)
		if err != nil {
			return err
		}
		free, err := hasFreeVars(formula)
		if err != nil {
			return err
		}
		if free {
			firstOpen = i
			break
		}
	}
	if firstOpen < 0 {
		return nil
	}
	preserveResult := e.backend == "duality" && e.requireResultSnapshot(EngineDuality) == nil
	prefix, err := e.removePendingEagerRules()
	if err != nil {
		return err
	}
	for i, first := firstOpen, true; i < len(e.rawRules); {
		raw := e.rawRules[i]
		if !raw.pending {
			i++
			continue
		}
		if !first {
			formula, err := e.manager.RefGet(raw.formula)
			if err != nil {
				return err
			}
			free, err := hasFreeVars(formula)
			if err != nil {
				return err
			}
			if !free {
				i++
				continue
			}
		}
		first = false
		err = e.normalizePendingOpenSourceRule(raw)
		length, lenErr := e.set.Len()
		if lenErr != nil {
			return errors.Join(err, lenErr)
		}
		if length != prefix || err == nil {
			e.advanceRuleRevision()
			if preserveResult {
				e.queryRevision = e.revision
			}
			prefix = length
		}
		if err != nil {
			return err
		}
		last := len(e.rawRules) - 1
		e.rawRules[i] = e.rawRules[last]
		// The source swaps formulas and names but only pops the bounds vector.
		e.rawRules[i].bound = raw.bound
		e.rawRules = e.rawRules[:last]
		e.retiredRawRules = append(e.retiredRawRules, raw)
		if err = e.cleanupRetiredRawRules(); err != nil {
			return err
		}
	}
	return nil
}

func (e *PropositionalEngine) normalizePendingOpenSourceRule(raw rawRuleFormula) (resultErr error) {
	// rule_manager::mk_rule scopes proof generation even when its proof input is nil.
	proofOwner := &pendingFlushProofOwner{manager: e.manager}
	defer func() { resultErr = proofOwner.finish(resultErr) }()
	enabled, err := e.pendingProofEnabled()
	if err != nil {
		return err
	}
	mode := ast.ASTProofDisabled
	if enabled {
		mode = ast.ASTProofEnabled
	}
	proofOwner.scope, err = ast.NewScopedProofMode(e.manager, mode)
	if err != nil {
		return err
	}
	a := &contextFormulaAttempt{manager: e.manager}
	defer func() { resultErr = a.finish(resultErr) }()
	formula, err := e.manager.RefGet(raw.formula)
	if err != nil {
		return err
	}
	bound, err := e.binder.Bind(formula, true)
	if bound != (ast.ASTRef{}) {
		a.references = append(a.references, bound)
	}
	if err != nil {
		return err
	}
	formula, err = e.manager.RefGet(bound)
	if err != nil {
		return err
	}
	return e.addFormula(e.set, formula, raw.name)
}
