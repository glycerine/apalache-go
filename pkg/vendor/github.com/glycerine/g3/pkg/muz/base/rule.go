package base

// Source: src/muz/base/dl_rule.h rule and rule_manager;
// src/muz/base/dl_rule.cpp rule_manager::mk, check_valid_head, and rule
// ownership/query operations.

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrRuleOwner = errors.New("muz/base: invalid or closed rule owner")

type ruleTail struct {
	handle    ast.ASTHandle
	reference ast.ASTRef
	negated   bool
}

type ruleState struct {
	manager       *ast.ASTManager
	head          ast.ASTHandle
	headReference ast.ASTRef
	tail          []ruleTail
	positive      int
	uninterpreted int
	name          string
	references    uint32
	accounting    *AccountedObject
	released      bool
}

// Rule is one owning reference to a Horn-rule representation. Tail order
// matches the native rule layout: positive predicates, negative predicates,
// then interpreted formulas.
type Rule struct {
	state  *ruleState
	closed bool
}

// RuleManager owns the registered predicate declarations used to classify
// rule heads and tails. Rules own their AST references independently.
type RuleManager struct {
	manager    *ast.ASTManager
	predicates map[ast.ASTHandle]ast.ASTRef
	closed     bool
}

func NewRuleManager(manager *ast.ASTManager) (*RuleManager, error) {
	if manager == nil {
		return nil, ErrRuleOwner
	}
	return &RuleManager{manager: manager, predicates: make(map[ast.ASTHandle]ast.ASTRef)}, nil
}

func (m *RuleManager) open() error {
	if m == nil || m.closed || m.manager == nil {
		return ErrRuleOwner
	}
	return nil
}

// RegisterPredicate retains a declaration as a recursive Horn predicate.
func (m *RuleManager) RegisterPredicate(declaration ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	kind, err := m.manager.GetKind(declaration)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return fmt.Errorf("muz/base: predicate is not a function declaration")
	}
	if _, found := m.predicates[declaration]; found {
		return nil
	}
	reference, err := m.manager.NewRef(declaration)
	if err != nil {
		return err
	}
	m.predicates[declaration] = reference
	return nil
}

func (m *RuleManager) isPredicate(application ast.ASTHandle) (bool, error) {
	kind, err := m.manager.GetKind(application)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, fmt.Errorf("muz/base: expected application in rule")
	}
	declaration, err := m.manager.GetDecl(application)
	if err != nil {
		return false, err
	}
	_, found := m.predicates[declaration]
	return found, nil
}

func (m *RuleManager) validHead(head ast.ASTHandle) error {
	predicate, err := m.isPredicate(head)
	if err != nil {
		return err
	}
	if !predicate {
		return fmt.Errorf("muz/base: illegal head; predicate must be registered")
	}
	count, err := m.manager.GetNumArgs(head)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		argument, e := m.manager.GetArg(head, i)
		if e != nil {
			return e
		}
		kind, e := m.manager.GetKind(argument)
		if e != nil {
			return e
		}
		if kind == ast.ASTVar {
			continue
		}
		value, e := m.manager.IsValue(argument)
		if e != nil {
			return e
		}
		if !value {
			return fmt.Errorf("muz/base: illegal non-variable argument in rule head")
		}
	}
	return nil
}

// NewRule constructs and owns a rule. negated may be nil; otherwise it must
// have one entry per tail. Negated interpreted formulas are materialized as
// Boolean not applications, matching the native representation.
func (m *RuleManager) NewRule(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool, name string) (_ *Rule, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if negated != nil && len(negated) != len(tail) {
		return nil, fmt.Errorf("muz/base: negation count does not match tail size")
	}
	if err := m.validHead(head); err != nil {
		return nil, err
	}
	r := &Rule{state: &ruleState{manager: m.manager, head: head, name: name, references: 1, accounting: &AccountedObject{}}}
	var err error
	r.state.headReference, err = m.manager.NewRef(head)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, r.Close())
		}
	}()
	uninterpreted := make([]ruleTail, 0, len(tail))
	interpreted := make([]ruleTail, 0, len(tail))
	for i, expression := range tail {
		isPredicate, e := m.isPredicate(expression)
		if e != nil {
			return nil, e
		}
		isNegated := negated != nil && negated[i]
		stored := expression
		if isNegated && !isPredicate {
			stored, e = m.manager.MkNot(expression)
			if e != nil {
				return nil, e
			}
			isNegated = false
		}
		reference, e := m.manager.NewRef(stored)
		if e != nil {
			return nil, e
		}
		entry := ruleTail{handle: stored, reference: reference, negated: isNegated}
		// Keep every acquired reference reachable by Close during construction.
		r.state.tail = append(r.state.tail, entry)
		if isPredicate {
			uninterpreted = append(uninterpreted, entry)
		} else {
			interpreted = append(interpreted, entry)
		}
	}
	// Native rule_manager::mk fills the interpreted segment downward.
	for left, right := 0, len(interpreted)-1; left < right; left, right = left+1, right-1 {
		interpreted[left], interpreted[right] = interpreted[right], interpreted[left]
	}
	// Mirror the source's in-place two-ended partition of predicate tails.
	end := len(uninterpreted)
	for i := 0; i < end; {
		if uninterpreted[i].negated {
			end--
			uninterpreted[i], uninterpreted[end] = uninterpreted[end], uninterpreted[i]
		} else {
			i++
		}
	}
	r.state.positive = end
	r.state.uninterpreted = len(uninterpreted)
	r.state.tail = append(uninterpreted, interpreted...)
	if err = m.normalizeVariables(r.state); err != nil {
		return nil, err
	}
	return r, nil
}

func (m *RuleManager) normalizeVariables(state *ruleState) (resultErr error) {
	used, err := ast.NewASTUsedVars(m.manager)
	if err != nil {
		return err
	}
	used.Reset()
	if err = used.Process(state.head); err != nil {
		return err
	}
	for _, entry := range state.tail {
		if err = used.Process(entry.handle); err != nil {
			return err
		}
	}
	count := used.MaxFoundVarIndexPlusOne()
	if used.UsesAllVars(count) {
		return nil
	}
	bindings := make([]ast.ASTHandle, count)
	var next uint32
	for i := uint32(0); i < count; i++ {
		sort := used.Contains(i)
		if sort.IsNull() {
			continue
		}
		bindings[i], err = m.manager.MkVar(next, sort)
		if err != nil {
			return err
		}
		next++
	}
	substitution, err := rewriter.NewVarSubstitution(m.manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return err
	}
	replace := func(handle *ast.ASTHandle, reference *ast.ASTRef) error {
		updated, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		if e = substitution.Substitute(*handle, bindings, updated); e != nil {
			return errors.Join(e, m.manager.DelRef(&updated))
		}
		value, e := m.manager.RefGet(updated)
		if e != nil {
			return errors.Join(e, m.manager.DelRef(&updated))
		}
		if e = m.manager.DelRef(reference); e != nil {
			return errors.Join(e, m.manager.DelRef(&updated))
		}
		*handle, *reference = value, updated
		return nil
	}
	if err = replace(&state.head, &state.headReference); err != nil {
		return err
	}
	for i := range state.tail {
		if err = replace(&state.tail[i].handle, &state.tail[i].reference); err != nil {
			return err
		}
	}
	return nil
}

func (r *Rule) open() (*ruleState, error) {
	if r == nil || r.closed || r.state == nil || r.state.released || r.state.manager == nil {
		return nil, ErrRuleOwner
	}
	return r.state, nil
}

// Retain returns a distinct owning reference to the same native rule value.
func (r *Rule) Retain() (*Rule, error) {
	state, err := r.open()
	if err != nil {
		return nil, err
	}
	if state.references == ^uint32(0) {
		return nil, ErrRuleOwner
	}
	state.references++
	return &Rule{state: state}, nil
}

// Same reports whether two owning references designate the same native rule.
func (r *Rule) Same(other *Rule) (bool, error) {
	state, err := r.open()
	if err != nil {
		return false, err
	}
	otherState, err := other.open()
	if err != nil {
		return false, err
	}
	return state == otherState, nil
}

func (r *Rule) Head() (ast.ASTHandle, error) {
	state, err := r.open()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.head, nil
}

// ASTManager returns the borrowed manager that owns this rule's expressions.
func (r *Rule) ASTManager() (*ast.ASTManager, error) {
	state, err := r.open()
	if err != nil {
		return nil, err
	}
	return state.manager, nil
}

// AccountingCosts returns the accumulated concrete instruction costs owned by
// this rule. Child accounted objects propagate their processed costs here.
func (r *Rule) AccountingCosts() (Costs, error) {
	state, err := r.open()
	if err != nil || state.accounting == nil {
		return Costs{}, errors.Join(ErrRuleOwner, err)
	}
	return state.accounting.TotalCosts()
}

// HeadDeclaration returns the borrowed predicate declaration of the rule head.
func (r *Rule) HeadDeclaration() (ast.ASTHandle, error) {
	state, err := r.open()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.GetDecl(state.head)
}

func (r *Rule) Name() (string, error) {
	state, err := r.open()
	if err != nil {
		return "", err
	}
	return state.name, nil
}

func (r *Rule) TailSize() (int, error) {
	state, err := r.open()
	if err != nil {
		return 0, err
	}
	return len(state.tail), nil
}

func (r *Rule) PositiveTailSize() (int, error) {
	state, err := r.open()
	if err != nil {
		return 0, err
	}
	return state.positive, nil
}

func (r *Rule) UninterpretedTailSize() (int, error) {
	state, err := r.open()
	if err != nil {
		return 0, err
	}
	return state.uninterpreted, nil
}

func (r *Rule) Tail(index int) (ast.ASTHandle, bool, error) {
	state, err := r.open()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if index < 0 || index >= len(state.tail) {
		return ast.ASTHandle{}, false, fmt.Errorf("muz/base: tail index out of bounds")
	}
	return state.tail[index].handle, state.tail[index].negated, nil
}

// TailDeclaration returns the borrowed predicate declaration at an
// uninterpreted-tail index.
func (r *Rule) TailDeclaration(index int) (ast.ASTHandle, error) {
	state, err := r.open()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if index < 0 || index >= state.uninterpreted {
		return ast.ASTHandle{}, fmt.Errorf("muz/base: uninterpreted tail index out of bounds")
	}
	return state.manager.GetDecl(state.tail[index].handle)
}

func (r *Rule) HasNegation() (bool, error) {
	state, err := r.open()
	if err != nil {
		return false, err
	}
	return state.positive != state.uninterpreted, nil
}

func (r *Rule) IsInTail(declaration ast.ASTHandle, onlyPositive bool) (bool, error) {
	state, err := r.open()
	if err != nil {
		return false, err
	}
	end := state.uninterpreted
	if onlyPositive {
		end = state.positive
	}
	for i := 0; i < end; i++ {
		candidate, err := state.manager.GetDecl(state.tail[i].handle)
		if err != nil {
			return false, err
		}
		if candidate == declaration {
			return true, nil
		}
	}
	return false, nil
}

// ToFormula converts the rule to its universally quantified Horn formula and
// returns one owning AST reference.
func (r *Rule) ToFormula() (result ast.ASTRef, resultErr error) {
	state, err := r.open()
	if err != nil {
		return result, err
	}
	body := make([]ast.ASTHandle, len(state.tail))
	for i, entry := range state.tail {
		body[i] = entry.handle
		if entry.negated {
			body[i], err = state.manager.MkNot(entry.handle)
			if err != nil {
				return result, err
			}
		}
	}
	formula := state.head
	switch len(body) {
	case 0:
	case 1:
		formula, err = state.manager.MkImplies(body[0], formula)
	default:
		var conjunction ast.ASTHandle
		conjunction, err = state.manager.MkAndArgs(body)
		if err == nil {
			formula, err = state.manager.MkImplies(conjunction, formula)
		}
	}
	if err != nil {
		return result, err
	}
	used, err := ast.NewASTUsedVars(state.manager)
	if err != nil {
		return result, err
	}
	if err = used.Collect(formula); err != nil {
		return result, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	if count != 0 {
		boolean, e := state.manager.MkBoolSort()
		if e != nil {
			return result, e
		}
		sorts := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			sort := used.Contains(i)
			if sort.IsNull() {
				sort = boolean
			}
			sorts[count-1-i] = sort
		}
		usedSymbols, e := ast.NewUsedSymbols(state.manager)
		if e != nil {
			return result, e
		}
		if e = usedSymbols.Collect(formula, false); e != nil {
			return result, e
		}
		names := make([]usymbol.Symbol, 0, count)
		for round := 0; uint32(len(names)) < count; round++ {
			for letter := 'A'; letter <= 'Z' && uint32(len(names)) < count; letter++ {
				spelling := string(letter)
				if round != 0 {
					spelling += strconv.Itoa(round)
				}
				name, x := state.manager.Symbols.Intern(spelling)
				if x != nil {
					return result, x
				}
				if usedSymbols.Contains(name) {
					continue
				}
				usedSymbols.Insert(name)
				names = append(names, name)
			}
		}
		formula, err = state.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula})
		if err != nil {
			return result, err
		}
	}
	return state.manager.NewRef(formula)
}

func (r *Rule) Close() (result error) {
	if r == nil || r.state == nil {
		return ErrRuleOwner
	}
	if r.closed {
		return nil
	}
	r.closed = true
	state := r.state
	if state.released || state.references == 0 {
		return ErrRuleOwner
	}
	state.references--
	if state.references != 0 {
		return nil
	}
	if state.accounting != nil {
		result = errors.Join(result, state.accounting.Close())
		state.accounting = nil
	}
	for i := len(state.tail) - 1; i >= 0; i-- {
		result = errors.Join(result, state.manager.DelRef(&state.tail[i].reference))
	}
	result = errors.Join(result, state.manager.DelRef(&state.headReference))
	state.tail = nil
	state.released = true
	return result
}

func (m *RuleManager) Close() (result error) {
	if m == nil || m.manager == nil {
		return ErrRuleOwner
	}
	if m.closed {
		return nil
	}
	for declaration, reference := range m.predicates {
		ref := reference
		result = errors.Join(result, m.manager.DelRef(&ref))
		delete(m.predicates, declaration)
	}
	m.closed = true
	return result
}
