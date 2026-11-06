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

// InvalidRuleHeadError retains the source query diagnostic after rule
// normalization releases its temporary expression.
type InvalidRuleHeadError struct{ SourceMessage string }

func (e *InvalidRuleHeadError) Error() string {
	return "muz/base: illegal head; predicate must be registered"
}

type ruleTail struct {
	handle    ast.ASTHandle
	reference ast.ASTRef
	negated   bool
}

type ruleState struct {
	manager        *ast.ASTManager
	head           ast.ASTHandle
	headReference  ast.ASTRef
	tail           []ruleTail
	positive       int
	uninterpreted  int
	name           usymbol.Symbol
	proofReference ast.ASTRef
	hasProof       bool
	references     uint32
	accounting     *AccountedObject
	released       bool
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
	manager        *ast.ASTManager
	normalizer     *HornNormalizer
	predicates     map[ast.ASTHandle]ast.ASTRef
	closed         bool
	cleanupPending bool
}

func NewRuleManager(manager *ast.ASTManager) (*RuleManager, error) {
	if manager == nil {
		return nil, ErrRuleOwner
	}
	normalizer, err := NewHornNormalizer(manager, "")
	if err != nil {
		return nil, err
	}
	return &RuleManager{manager: manager, normalizer: normalizer, predicates: make(map[ast.ASTHandle]ast.ASTRef)}, nil
}

func (m *RuleManager) open() error {
	if m == nil || m.closed || m.cleanupPending || m.manager == nil {
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
		display, err := m.ruleHeadSourceDisplay(head)
		if err != nil {
			return err
		}
		return &InvalidRuleHeadError{SourceMessage: "Illegal head. The head predicate needs to be uninterpreted and registered (as recursive) " + display}
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
	label, err := m.ruleName(name)
	if err != nil {
		return nil, err
	}
	return m.NewRuleWithSymbol(head, tail, negated, label)
}

// NewRuleWithSymbol preserves the source name without converting its identity
// to text. The symbol must belong to this AST manager or be null/numerical.
func (m *RuleManager) NewRuleWithSymbol(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool, name usymbol.Symbol) (*Rule, error) {
	return m.newRule(head, tail, negated, name, true)
}

func (m *RuleManager) newRule(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool, name usymbol.Symbol, normalize bool) (_ *Rule, resultErr error) {
	return m.newRuleWithHeadPolicy(head, tail, negated, name, normalize, true)
}

// Source rule_manager::substitute uses its internal mk path, which permits
// symbolic head arguments in release builds while retaining the predicate.
func (m *RuleManager) newRuleWithHeadPolicy(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool, name usymbol.Symbol, normalize, validateHead bool) (_ *Rule, resultErr error) {
	if err := m.checkRuleName(name); err != nil {
		return nil, err
	}
	if negated != nil && len(negated) != len(tail) {
		return nil, fmt.Errorf("muz/base: negation count does not match tail size")
	}
	if validateHead {
		if err := m.validHead(head); err != nil {
			return nil, err
		}
	} else {
		predicate, err := m.isPredicate(head)
		if err != nil {
			return nil, err
		}
		if !predicate {
			return nil, fmt.Errorf("muz/base: illegal head; predicate must be registered")
		}
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
		var reference ast.ASTRef
		if isNegated && !isPredicate {
			// Reserve ownership before constructing the fresh negation: acquiring
			// a reference afterwards can fail and strand the unowned AST node.
			reference, e = m.manager.NewRef(ast.ASTHandle{})
			if e != nil {
				return nil, e
			}
			// The partially built rule owns the slot even when a later step
			// fails, so its normal cleanup handles a retained reference.
			r.state.tail = append(r.state.tail, ruleTail{reference: reference})
			stored, e = m.manager.MkNot(expression)
			e = errors.Join(e, m.manager.SetRef(reference, stored))
			if e != nil {
				return nil, e
			}
			isNegated = false
		} else {
			reference, e = m.manager.NewRef(stored)
			if e != nil {
				return nil, e
			}
		}
		entry := ruleTail{handle: stored, reference: reference, negated: isNegated}
		// Keep every acquired reference reachable by Close during construction.
		if negated != nil && negated[i] && !isPredicate {
			r.state.tail[len(r.state.tail)-1] = entry
		} else {
			r.state.tail = append(r.state.tail, entry)
		}
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
	if normalize {
		if err = m.normalizeVariables(r.state); err != nil {
			return nil, err
		}
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
	// The source substitution vector owns each fresh variable until the
	// substitution is finished, including on allocation failure.
	var bindingRefs []ast.ASTRef
	defer func() {
		for i := len(bindingRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.manager.DelRef(&bindingRefs[i]))
		}
	}()
	var next uint32
	for i := uint32(0); i < count; i++ {
		sort := used.Contains(i)
		if sort.IsNull() {
			continue
		}
		owner, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		bindingRefs = append(bindingRefs, owner)
		bindings[i], err = m.manager.MkVar(next, sort)
		if err = errors.Join(err, m.manager.SetRef(owner, bindings[i])); err != nil {
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

// SetAccountingParentRule retains parent and propagates processed instruction
// costs to it, matching rule::set_accounting_parent_object.
func (r *Rule) SetAccountingParentRule(parent *Rule) error {
	state, err := r.open()
	if err != nil || state.accounting == nil {
		return errors.Join(ErrRuleOwner, err)
	}
	return state.accounting.SetAccountingParentRule(parent)
}

// HeadDeclaration returns the borrowed predicate declaration of the rule head.
func (r *Rule) HeadDeclaration() (ast.ASTHandle, error) {
	state, err := r.open()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.GetDecl(state.head)
}

// Name is the string convenience view; absent and present-empty labels both
// produce an empty string. Use NameSymbol for copying or comparing labels.
func (r *Rule) Name() (string, error) {
	state, err := r.open()
	if err != nil {
		return "", err
	}
	if state.name == (usymbol.Symbol{}) {
		return "", nil
	}
	return state.manager.Symbols.String(state.name)
}

// Proof returns the borrowed proof currently attached to this shared rule.
func (r *Rule) Proof() (ast.ASTHandle, bool, error) {
	state, err := r.open()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !state.hasProof {
		return ast.ASTHandle{}, false, nil
	}
	proof, err := state.manager.RefGet(state.proofReference)
	return proof, err == nil, err
}

// SetProof replaces the source rule's owning proof reference. Passing a null
// handle clears it, matching rule::set_proof(nullptr).
func (r *Rule) SetProof(proof ast.ASTHandle) (resultErr error) {
	state, err := r.open()
	if err != nil {
		return err
	}
	var replacement ast.ASTRef
	hasReplacement := !proof.IsNull()
	if hasReplacement {
		isProof, proofErr := state.manager.IsProof(proof)
		if proofErr != nil {
			return proofErr
		}
		if !isProof {
			return fmt.Errorf("muz/base: rule proof is not an AST proof")
		}
		replacement, err = state.manager.NewRef(proof)
		if err != nil {
			return err
		}
	}
	previous, hadPrevious := state.proofReference, state.hasProof
	state.proofReference = replacement
	state.hasProof = hasReplacement
	if hadPrevious {
		resultErr = state.manager.DelRef(&previous)
	}
	return resultErr
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
	// Source to_formula builds into an expr_ref and an expr_ref_vector body.
	// Reserve their owners before publishing any fresh Boolean/quantifier node.
	owner := &contextSMT2Owner{manager: state.manager}
	output := &contextSMT2Owner{manager: state.manager}
	defer func() {
		resultErr = owner.finish(resultErr)
		if resultErr != nil {
			resultErr = output.finish(resultErr)
			result = ast.ASTRef{}
		}
	}()
	result, err = state.manager.NewRef(state.head)
	if result != (ast.ASTRef{}) {
		output.refs = append(output.refs, result)
	}
	if err != nil {
		return result, err
	}
	body := make([]ast.ASTHandle, len(state.tail))
	for i, entry := range state.tail {
		body[i] = entry.handle
		if entry.negated {
			pin, e := owner.reserve()
			if e != nil {
				return result, e
			}
			body[i], err = state.manager.MkNot(entry.handle)
			if err = errors.Join(err, state.manager.SetRef(pin, body[i])); err != nil {
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
		pin, e := owner.reserve()
		if e != nil {
			return result, e
		}
		var conjunction ast.ASTHandle
		conjunction, err = state.manager.MkAndArgs(body)
		err = errors.Join(err, state.manager.SetRef(pin, conjunction))
		if err == nil {
			formula, err = state.manager.MkImplies(conjunction, formula)
		}
	}
	if err = errors.Join(err, state.manager.SetRef(result, formula)); err != nil {
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
		if err = errors.Join(err, state.manager.SetRef(result, formula)); err != nil {
			return result, err
		}
	}
	return result, nil
}

// Close releases a shared owner immediately, but final destruction preserves
// ownership when the manager or accounting owner refuses cleanup.
func (r *Rule) Close() error {
	if r == nil || r.state == nil {
		return ErrRuleOwner
	}
	if r.closed {
		return nil
	}
	state := r.state
	if state.released || state.references == 0 {
		return ErrRuleOwner
	}
	if state.references > 1 {
		state.references--
		r.closed = true
		return nil
	}
	if _, err := state.manager.ProofMode(); err != nil {
		return err
	}
	if state.accounting != nil {
		if err := state.accounting.Close(); err != nil {
			return err
		}
		state.accounting = nil
	}
	if state.hasProof {
		err := state.manager.DelRef(&state.proofReference)
		if state.proofReference == (ast.ASTRef{}) {
			state.hasProof = false
		}
		if err != nil {
			return err
		}
	}
	for len(state.tail) > 0 {
		last := len(state.tail) - 1
		err := state.manager.DelRef(&state.tail[last].reference)
		if state.tail[last].reference == (ast.ASTRef{}) {
			state.tail = state.tail[:last]
		}
		if err != nil {
			return err
		}
	}
	if state.headReference != (ast.ASTRef{}) {
		if err := state.manager.DelRef(&state.headReference); err != nil {
			return err
		}
	}
	state.references = 0
	state.released = true
	r.closed = true
	return nil
}

func (m *RuleManager) Close() error {
	if m == nil || m.manager == nil {
		return ErrRuleOwner
	}
	if m.closed {
		return nil
	}
	if _, err := m.manager.ProofMode(); err != nil {
		return err
	}
	m.cleanupPending = true
	if err := m.normalizer.Close(); err != nil {
		return err
	}
	for declaration, reference := range m.predicates {
		ref := reference
		err := m.manager.DelRef(&ref)
		if ref == (ast.ASTRef{}) {
			delete(m.predicates, declaration)
		} else {
			m.predicates[declaration] = ref
		}
		if err != nil {
			return err
		}
	}
	m.cleanupPending = false
	m.closed = true
	return nil
}
