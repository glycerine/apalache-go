package fp

// Source: src/muz/base/dl_context.cpp add_rule/query orchestration and
// src/muz/base/dl_engine_base.h query contract. This is the retained stratified
// finite-domain Horn execution path.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	muzpdr "github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/rel"
	muzspacer "github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var (
	ErrPropositionalEngine = errors.New("muz/fp: invalid or closed propositional engine")
	ErrPropositionalHorn   = errors.New("muz/fp: positive finite-domain Horn fragment required")
)

type predicateState struct {
	sorts []ast.ASTHandle
	table *rel.HashTable
}

func (e *PropositionalEngine) predicateState(declaration ast.ASTHandle) (*predicateState, error) {
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		sorts[index], err = e.manager.GetDomain(declaration, index)
		if err != nil {
			return nil, err
		}
	}
	table, tableErr := e.relations.GetRelation(declaration)
	if tableErr != nil && !errors.Is(tableErr, rel.ErrTable) {
		return nil, tableErr
	}
	return &predicateState{sorts: sorts, table: table}, nil
}

// PropositionalEngine owns the retained rule, finite-relation, and optional
// ground-guard rewriting services.
type PropositionalEngine struct {
	manager            *ast.ASTManager
	rules              *base.RuleManager
	set                *base.RuleSet
	binder             *base.VariableBinder
	relations          *rel.RelationManager
	guard              *astrewriter.TheoryRewriter
	background         ast.ASTRef
	params             *uparams.ParamsContext
	globals            *uparams.GlobalParamsService
	backend            string
	pdr                *muzpdr.Context
	spacer             *muzspacer.Context
	scopes             []*base.RuleSet
	predicates         map[ast.ASTHandle]*predicateState
	predicateOrder     []ast.ASTHandle
	auxiliary          map[ast.ASTHandle]bool
	compiled           bool
	queried            bool
	revision           uint64
	pdrRevision        uint64
	spacerRevision     uint64
	spacerPredicates   map[ast.ASTHandle]ast.ASTHandle
	spacerSlices       map[ast.ASTHandle][]bool
	spacerQuantified   map[ast.ASTHandle]ast.ASTHandle
	spacerInstantiated map[ast.ASTHandle]ast.ASTHandle
	spacerBitBlasted   map[ast.ASTHandle]ast.ASTHandle
	spacerScaled       map[ast.ASTHandle]ast.ASTHandle
	spacerMagic        map[ast.ASTHandle]ast.ASTHandle
	spacerKarr         map[ast.ASTHandle]rel.KarrMatrix
	closed             bool
}

// NewPropositionalEngineWithServices adds the retained AST theory rewriter
// used to evaluate ground interpreted guards after finite variable binding.
func NewPropositionalEngineWithServices(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*PropositionalEngine, error) {
	engine, err := NewPropositionalEngine(manager)
	if err != nil {
		return nil, err
	}
	engine.guard, err = astrewriter.NewTheoryRewriter(manager, context, globals, nil)
	if err != nil {
		return nil, errors.Join(err, engine.Close())
	}
	engine.params = context
	engine.globals = globals
	return engine, nil
}

func NewPropositionalEngine(manager *ast.ASTManager) (*PropositionalEngine, error) {
	if manager == nil {
		return nil, ErrPropositionalEngine
	}
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		return nil, err
	}
	set, err := base.NewRuleSet(rules)
	if err != nil {
		return nil, errors.Join(err, rules.Close())
	}
	binder, err := base.NewVariableBinder(manager)
	if err != nil {
		return nil, errors.Join(err, set.Close(), rules.Close())
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		return nil, errors.Join(err, binder.Close(), set.Close(), rules.Close())
	}
	truth, err := manager.MkTrue()
	if err != nil {
		return nil, errors.Join(err, relations.Close(), binder.Close(), set.Close(), rules.Close())
	}
	background, err := manager.NewRef(truth)
	if err != nil {
		return nil, errors.Join(err, relations.Close(), binder.Close(), set.Close(), rules.Close())
	}
	return &PropositionalEngine{
		manager: manager, rules: rules, set: set, binder: binder, relations: relations,
		background: background, predicates: make(map[ast.ASTHandle]*predicateState), auxiliary: make(map[ast.ASTHandle]bool), backend: "datalog", revision: 1,
	}, nil
}

func (e *PropositionalEngine) open() error {
	if e == nil || e.closed || e.manager == nil || e.rules == nil || e.set == nil || e.binder == nil || e.relations == nil {
		return ErrPropositionalEngine
	}
	return nil
}

func (e *PropositionalEngine) RegisterPredicate(declaration ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	kind, err := e.manager.GetKind(declaration)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return ErrPropositionalHorn
	}
	rangeSort, err := e.manager.GetRange(declaration)
	if err != nil {
		return err
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if rangeSort != boolean {
		return ErrPropositionalHorn
	}
	if err = e.rules.RegisterPredicate(declaration); err != nil {
		return err
	}
	state, err := e.predicateState(declaration)
	if err != nil {
		return err
	}
	if e.predicates[declaration] == nil {
		e.predicateOrder = append(e.predicateOrder, declaration)
	}
	e.predicates[declaration] = state
	e.revision++
	return nil
}

// RegisterVariable retains an implicit Datalog variable application.
func (e *PropositionalEngine) RegisterVariable(variable ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if err := e.binder.AddVariable(variable); err != nil {
		return err
	}
	e.revision++
	return nil
}

// SetBackground replaces the background constraint used by subsequent PDR
// queries. The engine retains the formula independently of the caller.
func (e *PropositionalEngine) SetBackground(background ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	current, err := e.manager.RefGet(e.background)
	if err != nil {
		return err
	}
	if current == background {
		return nil
	}
	if err = e.manager.SetRef(e.background, background); err != nil {
		return err
	}
	e.revision++
	return nil
}

// ParametersUpdated invalidates a retained backend context so the next query
// reconstructs it from current fixedpoint parameters.
func (e *PropositionalEngine) ParametersUpdated() error {
	if err := e.open(); err != nil {
		return err
	}
	e.revision++
	return nil
}

func (e *PropositionalEngine) AddRule(formula ast.ASTHandle, name string) error {
	if err := e.open(); err != nil {
		return err
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	bound, err := e.binder.Bind(formula, true)
	if err != nil {
		return err
	}
	boundFormula, getErr := e.manager.RefGet(bound)
	if getErr == nil {
		getErr = e.rules.AddFormula(e.set, boundFormula, name)
	}
	added := getErr == nil
	getErr = errors.Join(getErr, e.manager.DelRef(&bound))
	if added {
		e.revision++
	}
	return getErr
}

// Push retains a snapshot of the current rule set. Predicate registrations are
// intentionally not scoped, matching dl_context::push.
func (e *PropositionalEngine) Push() error {
	if err := e.open(); err != nil {
		return err
	}
	snapshot, err := base.NewRuleSet(e.rules)
	if err != nil {
		return err
	}
	if err = snapshot.AddRules(e.set); err != nil {
		return errors.Join(err, snapshot.Close())
	}
	e.scopes = append(e.scopes, snapshot)
	return nil
}

// Pop restores the most recent rule snapshot. The native Datalog engine only
// permits this before a query creates its execution engine.
func (e *PropositionalEngine) Pop() error {
	if err := e.open(); err != nil {
		return err
	}
	if len(e.scopes) == 0 {
		return fmt.Errorf("muz/fp: there are no backtracking points to pop to")
	}
	if e.queried {
		return fmt.Errorf("muz/fp: pop operation is only supported before a query")
	}
	last := len(e.scopes) - 1
	snapshot := e.scopes[last]
	if err := e.set.ReplaceRules(snapshot); err != nil {
		return err
	}
	e.revision++
	e.scopes = e.scopes[:last]
	return snapshot.Close()
}

func (e *PropositionalEngine) hasUninterpretedRuleSymbol(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if seen[expression] {
			continue
		}
		seen[expression] = true
		kind, err := e.manager.GetKind(expression)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		declaration, err := e.manager.GetDecl(expression)
		if err != nil {
			return false, err
		}
		family, err := e.manager.GetFamilyID(declaration)
		if err != nil {
			return false, err
		}
		if family == ast.NullFamilyID && e.predicates[declaration] == nil {
			return true, nil
		}
		arguments, err := e.manager.GetNumArgs(expression)
		if err != nil {
			return false, err
		}
		datatypeFamily, datatypeErr := e.manager.DatatypeFamilyID()
		if datatypeErr == nil && family == datatypeFamily && arguments != 0 {
			declarationKind, x := e.manager.GetDeclKind(declaration)
			if x != nil {
				return false, x
			}
			if declarationKind == ast.ASTDatatypeAccessor {
				argument, x := e.manager.GetArg(expression, 0)
				if x != nil {
					return false, x
				}
				sort, x := e.manager.GetSort(argument)
				if x != nil {
					return false, x
				}
				constructors, x := e.manager.DatatypeConstructors(sort)
				if x != nil {
					return false, x
				}
				if len(constructors) > 1 {
					return true, nil
				}
			}
		}
		for index := uint32(0); index < arguments; index++ {
			argument, x := e.manager.GetArg(expression, index)
			if x != nil {
				return false, x
			}
			stack = append(stack, argument)
		}
	}
	return false, nil
}

func (e *PropositionalEngine) validateFragment() error {
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		rule, x := e.set.Rule(i)
		if x != nil {
			return x
		}
		head, x := rule.HeadDeclaration()
		if x == nil && e.predicates[head] == nil {
			x = ErrPropositionalHorn
		}
		if x == nil && e.backend == "datalog" {
			arity, y := e.manager.GetArity(head)
			for column := uint32(0); y == nil && column < arity; column++ {
				sort, getErr := e.manager.GetDomain(head, column)
				if getErr != nil {
					y = getErr
					break
				}
				size, getErr := e.manager.GetSortSize(sort)
				if getErr != nil {
					y = getErr
					break
				}
				ruleSort, getErr := e.manager.IsDatalogRuleSort(sort)
				if getErr != nil {
					y = getErr
					break
				}
				if !size.IsFinite() && !ruleSort {
					y = fmt.Errorf("%w: rule contains an infinite predicate sort", ErrPropositionalHorn)
				}
			}
			x = errors.Join(x, y)
		}
		positive, pErr := rule.PositiveTailSize()
		uninterpreted, uErr := rule.UninterpretedTailSize()
		tailSize, tErr := rule.TailSize()
		x = errors.Join(x, pErr, uErr, tErr)
		if x == nil && (positive > uninterpreted || uninterpreted > tailSize) {
			x = ErrPropositionalHorn
		}
		if x == nil && (e.backend == "pdr" || e.backend == "spacer") && positive != uninterpreted {
			x = fmt.Errorf("%w: rule contains a negative predicate", ErrPropositionalHorn)
		}
		for j := 0; x == nil && j < uninterpreted; j++ {
			declaration, y := rule.TailDeclaration(j)
			if y != nil {
				x = y
				break
			}
			if e.predicates[declaration] == nil {
				x = ErrPropositionalHorn
			}
		}
		for j := uninterpreted; x == nil && j < tailSize; j++ {
			tail, _, y := rule.Tail(j)
			if y != nil {
				x = y
				break
			}
			if e.backend == "datalog" {
				quantified, quantifierErr := e.manager.HasQuantifiers(tail)
				if quantifierErr != nil {
					x = quantifierErr
					break
				}
				if quantified {
					x = fmt.Errorf("%w: cannot process a quantifier in a Datalog rule", ErrPropositionalHorn)
					break
				}
			}
			unsupported, y := e.hasUninterpretedRuleSymbol(tail)
			if y != nil {
				x = y
				break
			}
			if unsupported {
				x = fmt.Errorf("%w: uninterpreted symbols in PDR rules are unsupported", ErrPropositionalHorn)
			}
		}
		x = errors.Join(x, rule.Close())
		if x != nil {
			return x
		}
	}
	return nil
}

func (e *PropositionalEngine) normalizeSimpleJoins() (resultErr error) {
	plugin := transforms.NewSimpleJoinsPlugin(e.manager, e.rules, 0)
	transformer := base.NewRuleTransformer()
	if err := transformer.Register(plugin); err != nil {
		return errors.Join(err, transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(e.set)
	if err != nil || !modified {
		return err
	}
	for _, declaration := range plugin.FreshPredicates() {
		state, x := e.predicateState(declaration)
		if x != nil {
			return x
		}
		e.predicates[declaration] = state
		e.auxiliary[declaration] = true
	}
	e.revision++
	return nil
}

type variableBinding map[uint32]rel.Element

func cloneBinding(binding variableBinding) variableBinding {
	result := make(variableBinding, len(binding))
	for variable, value := range binding {
		result[variable] = value
	}
	return result
}

func (e *PropositionalEngine) unify(application ast.ASTHandle, state *predicateState, fact rel.Fact, binding variableBinding) (variableBinding, bool, error) {
	result := cloneBinding(binding)
	for i, value := range fact {
		argument, err := e.manager.GetArg(application, uint32(i))
		if err != nil {
			return nil, false, err
		}
		kind, err := e.manager.GetKind(argument)
		if err != nil {
			return nil, false, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(argument)
			if x != nil {
				return nil, false, x
			}
			if previous, found := result[index]; found && previous != value {
				return nil, false, nil
			}
			result[index] = value
			continue
		}
		constant, err := e.relations.EncodeValue(state.sorts[i], argument)
		if err != nil {
			return nil, false, ErrPropositionalHorn
		}
		if constant != value {
			return nil, false, nil
		}
	}
	return result, true, nil
}

func (e *PropositionalEngine) instantiate(application ast.ASTHandle, state *predicateState, binding variableBinding) (rel.Fact, error) {
	fact := make(rel.Fact, len(state.sorts))
	for i, sort := range state.sorts {
		argument, err := e.manager.GetArg(application, uint32(i))
		if err != nil {
			return nil, err
		}
		kind, err := e.manager.GetKind(argument)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(argument)
			if x != nil {
				return nil, x
			}
			value, found := binding[index]
			if !found {
				return nil, fmt.Errorf("%w: head variable is not bound by a positive tail", ErrPropositionalHorn)
			}
			fact[i] = value
			continue
		}
		fact[i], err = e.relations.EncodeValue(sort, argument)
		if err != nil {
			return nil, ErrPropositionalHorn
		}
	}
	return fact, nil
}

func (e *PropositionalEngine) negativeMatch(application ast.ASTHandle, state *predicateState, fact rel.Fact, binding variableBinding) (bool, error) {
	for i, value := range fact {
		argument, err := e.manager.GetArg(application, uint32(i))
		if err != nil {
			return false, err
		}
		kind, err := e.manager.GetKind(argument)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(argument)
			if x != nil {
				return false, x
			}
			bound, found := binding[index]
			if !found {
				return false, fmt.Errorf("%w: negative-tail variable is not bound by a positive tail", ErrPropositionalHorn)
			}
			if bound != value {
				return false, nil
			}
			continue
		}
		constant, err := e.relations.EncodeValue(state.sorts[i], argument)
		if err != nil {
			return false, ErrPropositionalHorn
		}
		if constant != value {
			return false, nil
		}
	}
	return true, nil
}

func (e *PropositionalEngine) expandUnbound(application ast.ASTHandle, state *predicateState, bindings []variableBinding) ([]variableBinding, error) {
	signature, err := state.table.Signature()
	if err != nil {
		return nil, err
	}
	for i := range state.sorts {
		argument, x := e.manager.GetArg(application, uint32(i))
		if x != nil {
			return nil, x
		}
		kind, x := e.manager.GetKind(argument)
		if x != nil {
			return nil, x
		}
		if kind != ast.ASTVar {
			continue
		}
		index, x := e.manager.GetVarIndex(argument)
		if x != nil {
			return nil, x
		}
		domain, x := signature.Domain(i)
		if x != nil {
			return nil, x
		}
		next := make([]variableBinding, 0)
		for _, binding := range bindings {
			if _, found := binding[index]; found {
				next = append(next, binding)
				continue
			}
			for value := uint64(0); value < domain; value++ {
				expanded := cloneBinding(binding)
				expanded[index] = rel.Element(value)
				next = append(next, expanded)
			}
		}
		bindings = next
	}
	return bindings, nil
}

func (e *PropositionalEngine) expressionVariableSorts(root ast.ASTHandle) (map[uint32]ast.ASTHandle, error) {
	variables := make(map[uint32]ast.ASTHandle)
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		kind, err := e.manager.GetKind(current)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(current)
			if x != nil {
				return nil, x
			}
			if _, found := variables[index]; !found {
				variables[index], x = e.manager.GetSort(current)
				if x != nil {
					return nil, x
				}
			}
			continue
		}
		if kind != ast.ASTApp {
			return nil, ErrPropositionalHorn
		}
		count, err := e.manager.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		for i := count; i > 0; i-- {
			argument, x := e.manager.GetArg(current, i-1)
			if x != nil {
				return nil, x
			}
			stack = append(stack, argument)
		}
	}
	return variables, nil
}

func (e *PropositionalEngine) expandExpressionVariables(root ast.ASTHandle, bindings []variableBinding) ([]variableBinding, error) {
	variables, err := e.expressionVariableSorts(root)
	if err != nil {
		return nil, err
	}
	indices := make([]uint32, 0, len(variables))
	for index := range variables {
		indices = append(indices, index)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	for _, index := range indices {
		size, err := e.manager.GetSortSize(variables[index])
		if err != nil || !size.IsFinite() {
			return nil, ErrPropositionalHorn
		}
		domain, err := size.Size()
		if err != nil {
			return nil, err
		}
		next := make([]variableBinding, 0)
		for _, binding := range bindings {
			if _, found := binding[index]; found {
				next = append(next, binding)
				continue
			}
			for value := uint64(0); value < domain; value++ {
				expanded := cloneBinding(binding)
				expanded[index] = rel.Element(value)
				next = append(next, expanded)
			}
		}
		bindings = next
	}
	return bindings, nil
}

func (e *PropositionalEngine) finiteValue(expression ast.ASTHandle, binding variableBinding) (rel.Element, error) {
	kind, err := e.manager.GetKind(expression)
	if err != nil {
		return 0, err
	}
	if kind == ast.ASTVar {
		index, x := e.manager.GetVarIndex(expression)
		if x != nil {
			return 0, x
		}
		value, found := binding[index]
		if !found {
			return 0, ErrPropositionalHorn
		}
		return value, nil
	}
	sort, err := e.manager.GetSort(expression)
	if err != nil {
		return 0, err
	}
	value, err := e.relations.EncodeValue(sort, expression)
	if err != nil {
		return 0, ErrPropositionalHorn
	}
	return value, nil
}

func (e *PropositionalEngine) rewriteCondition(expression ast.ASTHandle, binding variableBinding) (value bool, result error) {
	if e.guard == nil {
		return false, ErrPropositionalHorn
	}
	variables, err := e.expressionVariableSorts(expression)
	if err != nil {
		return false, err
	}
	var maximum uint32
	for index := range variables {
		if index > maximum {
			maximum = index
		}
	}
	bindings := make([]ast.ASTHandle, 0)
	if len(variables) != 0 {
		bindings = make([]ast.ASTHandle, int(maximum)+1)
	}
	references := make([]ast.ASTRef, 0, len(variables)+1)
	defer func() {
		for i := len(references) - 1; i >= 0; i-- {
			result = errors.Join(result, e.manager.DelRef(&references[i]))
		}
	}()
	for index, sort := range variables {
		element, found := binding[index]
		if !found {
			return false, ErrPropositionalHorn
		}
		reference, x := e.relations.DecodeValue(sort, element)
		if x != nil {
			return false, x
		}
		references = append(references, reference)
		handle, x := e.manager.RefGet(reference)
		if x != nil {
			return false, x
		}
		bindings[len(bindings)-int(index)-1] = handle
	}
	out, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	references = append(references, out)
	if err = e.guard.RewriteWithBindings(expression, bindings, out); err != nil {
		return false, err
	}
	rewritten, err := e.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	if e.manager.IsTrue(rewritten) {
		return true, nil
	}
	if e.manager.IsFalse(rewritten) {
		return false, nil
	}
	return false, ErrPropositionalHorn
}

func (e *PropositionalEngine) evaluateCondition(expression ast.ASTHandle, binding variableBinding) (bool, error) {
	value, err := e.evaluateConditionDirect(expression, binding)
	if errors.Is(err, ErrPropositionalHorn) && e.guard != nil {
		return e.rewriteCondition(expression, binding)
	}
	return value, err
}

func (e *PropositionalEngine) evaluateConditionDirect(expression ast.ASTHandle, binding variableBinding) (bool, error) {
	if e.manager.IsTrue(expression) {
		return true, nil
	}
	if e.manager.IsFalse(expression) {
		return false, nil
	}
	kind, err := e.manager.GetKind(expression)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		value, x := e.finiteValue(expression, binding)
		return value != 0, x
	}
	if kind != ast.ASTApp {
		return false, ErrPropositionalHorn
	}
	operator, err := e.manager.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	count, err := e.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	argument := func(index uint32) (ast.ASTHandle, error) { return e.manager.GetArg(expression, index) }
	switch operator {
	case ast.ASTOpNot:
		if count != 1 {
			return false, ErrPropositionalHorn
		}
		child, x := argument(0)
		if x != nil {
			return false, x
		}
		value, x := e.evaluateCondition(child, binding)
		return !value, x
	case ast.ASTOpAnd, ast.ASTOpOr:
		result := operator == ast.ASTOpAnd
		for i := uint32(0); i < count; i++ {
			child, x := argument(i)
			if x != nil {
				return false, x
			}
			value, x := e.evaluateCondition(child, binding)
			if x != nil {
				return false, x
			}
			if operator == ast.ASTOpAnd {
				result = result && value
			} else {
				result = result || value
			}
		}
		return result, nil
	case ast.ASTOpEq, ast.ASTOpOEq:
		if count != 2 {
			return false, ErrPropositionalHorn
		}
		left, x := argument(0)
		if x != nil {
			return false, x
		}
		right, x := argument(1)
		if x != nil {
			return false, x
		}
		leftSort, x := e.manager.GetSort(left)
		if x != nil {
			return false, x
		}
		boolean, x := e.manager.MkBoolSort()
		if x != nil {
			return false, x
		}
		if leftSort == boolean {
			leftValue, y := e.evaluateCondition(left, binding)
			if y != nil {
				return false, y
			}
			rightValue, y := e.evaluateCondition(right, binding)
			return leftValue == rightValue, y
		}
		leftValue, x := e.finiteValue(left, binding)
		if x != nil {
			return false, x
		}
		rightValue, x := e.finiteValue(right, binding)
		return leftValue == rightValue, x
	case ast.ASTOpDistinct:
		seen := make(map[rel.Element]bool)
		for i := uint32(0); i < count; i++ {
			child, x := argument(i)
			if x != nil {
				return false, x
			}
			value, x := e.finiteValue(child, binding)
			if x != nil {
				return false, x
			}
			if seen[value] {
				return false, nil
			}
			seen[value] = true
		}
		return true, nil
	case ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpImplies:
		if count != 2 {
			return false, ErrPropositionalHorn
		}
		left, x := argument(0)
		if x != nil {
			return false, x
		}
		right, x := argument(1)
		if x != nil {
			return false, x
		}
		leftValue, x := e.evaluateCondition(left, binding)
		if x != nil {
			return false, x
		}
		rightValue, x := e.evaluateCondition(right, binding)
		if x != nil {
			return false, x
		}
		if operator == ast.ASTOpIff {
			return leftValue == rightValue, nil
		}
		if operator == ast.ASTOpXor {
			return leftValue != rightValue, nil
		}
		return !leftValue || rightValue, nil
	case ast.ASTOpIte:
		if count != 3 {
			return false, ErrPropositionalHorn
		}
		condition, x := argument(0)
		if x != nil {
			return false, x
		}
		branch, x := e.evaluateCondition(condition, binding)
		if x != nil {
			return false, x
		}
		index := uint32(2)
		if branch {
			index = 1
		}
		selected, x := argument(index)
		if x != nil {
			return false, x
		}
		return e.evaluateCondition(selected, binding)
	}
	sort, err := e.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	boolean, err := e.manager.MkBoolSort()
	if err == nil && sort == boolean {
		value, x := e.finiteValue(expression, binding)
		return value != 0, x
	}
	return false, ErrPropositionalHorn
}

func (e *PropositionalEngine) evaluateRule(rule *base.Rule) (changed bool, result error) {
	defer func() { result = errors.Join(result, rule.Close()) }()
	bindings := []variableBinding{{}}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	for i := 0; i < positive; i++ {
		application, negated, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		if negated {
			return false, ErrPropositionalHorn
		}
		declaration, x := e.manager.GetDecl(application)
		if x != nil {
			return false, x
		}
		state := e.predicates[declaration]
		if state == nil {
			return false, ErrPropositionalHorn
		}
		facts, x := state.table.Facts()
		if x != nil {
			return false, x
		}
		next := make([]variableBinding, 0)
		for _, binding := range bindings {
			for _, fact := range facts {
				unified, matches, y := e.unify(application, state, fact, binding)
				if y != nil {
					return false, y
				}
				if matches {
					next = append(next, unified)
				}
			}
		}
		bindings = next
	}
	for i := positive; i < uninterpreted; i++ {
		application, negated, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		if !negated {
			return false, ErrPropositionalHorn
		}
		declaration, x := e.manager.GetDecl(application)
		if x != nil {
			return false, x
		}
		state := e.predicates[declaration]
		if state == nil {
			return false, ErrPropositionalHorn
		}
		bindings, x = e.expandUnbound(application, state, bindings)
		if x != nil {
			return false, x
		}
		facts, x := state.table.Facts()
		if x != nil {
			return false, x
		}
		next := make([]variableBinding, 0, len(bindings))
		for _, binding := range bindings {
			excluded := false
			for _, fact := range facts {
				matches, y := e.negativeMatch(application, state, fact, binding)
				if y != nil {
					return false, y
				}
				if matches {
					excluded = true
					break
				}
			}
			if !excluded {
				next = append(next, binding)
			}
		}
		bindings = next
	}
	tails, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	for i := uninterpreted; i < tails; i++ {
		condition, negated, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		if negated {
			return false, ErrPropositionalHorn
		}
		bindings, x = e.expandExpressionVariables(condition, bindings)
		if x != nil {
			return false, x
		}
		next := make([]variableBinding, 0, len(bindings))
		for _, binding := range bindings {
			accepted, y := e.evaluateCondition(condition, binding)
			if y != nil {
				return false, y
			}
			if accepted {
				next = append(next, binding)
			}
		}
		bindings = next
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	declaration, err := e.manager.GetDecl(head)
	if err != nil {
		return false, err
	}
	state := e.predicates[declaration]
	if state == nil {
		return false, ErrPropositionalHorn
	}
	bindings, err = e.expandUnbound(head, state, bindings)
	if err != nil {
		return false, err
	}
	for _, binding := range bindings {
		fact, x := e.instantiate(head, state, binding)
		if x != nil {
			return false, x
		}
		added, x := state.table.Add(fact)
		if x != nil {
			return false, x
		}
		changed = changed || added
	}
	return changed, nil
}

func (e *PropositionalEngine) saturate() error {
	for _, state := range e.predicates {
		if err := state.table.Reset(); err != nil {
			return err
		}
	}
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	rulesByStratum := make(map[uint32][]int)
	var maximum uint32
	for i := 0; i < count; i++ {
		rule, x := e.set.Rule(i)
		if x != nil {
			return x
		}
		head, x := rule.HeadDeclaration()
		if x == nil {
			var stratum uint32
			stratum, x = e.set.PredicateStratum(head)
			if x == nil {
				rulesByStratum[stratum] = append(rulesByStratum[stratum], i)
				if stratum > maximum {
					maximum = stratum
				}
			}
		}
		x = errors.Join(x, rule.Close())
		if x != nil {
			return x
		}
	}
	for stratum := uint32(0); stratum <= maximum; stratum++ {
		for {
			changed := false
			for _, index := range rulesByStratum[stratum] {
				rule, x := e.set.Rule(index)
				if x != nil {
					return x
				}
				added, x := e.evaluateRule(rule)
				if x != nil {
					return x
				}
				changed = changed || added
			}
			if !changed {
				break
			}
		}
	}
	return nil
}

func (e *PropositionalEngine) tryCompiledSaturation() (used bool, resultErr error) {
	execution, termination, err := rel.CompileFiniteRulesWithGuard(e.relations, e.set, e.guard)
	if errors.Is(err, rel.ErrCompilerUnsupported) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	context, err := rel.NewExecutionContextWithRelations(e.relations, nil)
	if err != nil {
		return true, errors.Join(err, execution.Close(), termination.Close())
	}
	defer func() {
		resultErr = errors.Join(resultErr, execution.Close(), termination.Close(), context.Close())
	}()
	complete, err := execution.Perform(context)
	if err != nil {
		return true, err
	}
	terminated, terminationErr := termination.Perform(context)
	if terminationErr != nil {
		return true, terminationErr
	}
	if !complete || !terminated {
		return true, ErrPropositionalEngine
	}
	if err = execution.ProcessAllCosts(); err != nil {
		return true, err
	}
	for declaration, state := range e.predicates {
		table, found, x := e.relations.TryGetRelation(declaration)
		if x != nil {
			return true, x
		}
		if found {
			state.table = table
		}
	}
	return true, nil
}

// LastQueryUsedCompiledCode reports whether the most recent query ran the
// relational instruction compiler instead of the direct finite evaluator.
func (e *PropositionalEngine) LastQueryUsedCompiledCode() (bool, error) {
	if err := e.open(); err != nil {
		return false, err
	}
	return e.compiled, nil
}

func (e *PropositionalEngine) CollectStatistics(destination *statistics.Statistics) error {
	if err := e.open(); err != nil {
		return err
	}
	if destination == nil {
		return ErrPropositionalEngine
	}
	if e.backend == "pdr" && e.pdr != nil {
		return e.pdr.CollectStatistics(destination)
	}
	if e.backend == "spacer" && e.spacer != nil {
		return e.spacer.CollectStatistics(destination)
	}
	return nil
}

func (e *PropositionalEngine) ResetStatistics() error {
	if err := e.open(); err != nil {
		return err
	}
	if e.backend == "pdr" && e.pdr != nil {
		return e.pdr.ResetStatistics()
	}
	if e.backend == "spacer" && e.spacer != nil {
		return e.spacer.ResetStatistics()
	}
	return nil
}

// QueryPredicate returns true exactly when the registered predicate relation
// contains at least one tuple after positive Horn saturation.
func (e *PropositionalEngine) QueryPredicate(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	state := e.predicates[declaration]
	if state == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	e.queried = true
	if e.backend == "pdr" {
		e.compiled = false
		return e.queryPDR(declaration)
	}
	if e.backend == "spacer" {
		e.compiled = false
		return e.querySpacer(declaration)
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, x := e.set.Finalize()
		if x != nil {
			return lbool.Undef, x
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	if err = e.normalizeSimpleJoins(); err != nil {
		return lbool.Undef, err
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	if state.table == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	e.compiled, err = e.tryCompiledSaturation()
	if err != nil {
		return lbool.Undef, err
	}
	if !e.compiled {
		err = e.saturate()
	}
	if err != nil {
		return lbool.Undef, err
	}
	count, err := state.table.Len()
	if err != nil {
		return lbool.Undef, err
	}
	return lbool.FromBool(count != 0), nil
}

// QueryPredicateFromLevel runs a Spacer query beginning at the requested proof
// level.
func (e *PropositionalEngine) QueryPredicateFromLevel(declaration ast.ASTHandle, level uint32) (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if e.backend != "spacer" || e.predicates[declaration] == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	e.queried = true
	e.compiled = false
	return e.querySpacerFromLevel(declaration, level)
}

// Answer returns an owning formula describing the current finite relation.
// Each free variable corresponds to the predicate column with the same index.
func (e *PropositionalEngine) Answer(declaration ast.ASTHandle) (answer ast.ASTRef, result error) {
	if err := e.open(); err != nil {
		return answer, err
	}
	if e.backend == "pdr" {
		if e.pdr == nil || !e.queried {
			return answer, ErrPropositionalEngine
		}
		return e.pdr.Answer(declaration)
	}
	if e.backend == "spacer" {
		if e.spacer == nil || !e.queried || e.spacerRevision != e.revision {
			return answer, ErrPropositionalEngine
		}
		answer, result = e.spacer.Answer(e.spacerPredicate(declaration))
		if result != nil {
			return answer, result
		}
		return e.expandSpacerFormula(declaration, answer)
	}
	state := e.predicates[declaration]
	if state == nil || !e.queried {
		return answer, ErrPropositionalEngine
	}
	facts, err := state.table.Facts()
	if err != nil {
		return answer, err
	}
	temporaries := make([]ast.ASTRef, 0)
	defer func() {
		for i := len(temporaries) - 1; i >= 0; i-- {
			result = errors.Join(result, e.manager.DelRef(&temporaries[i]))
		}
		if result != nil && answer != (ast.ASTRef{}) {
			result = errors.Join(result, e.manager.DelRef(&answer))
			answer = ast.ASTRef{}
		}
	}()
	retain := func(handle ast.ASTHandle) (ast.ASTHandle, error) {
		reference, x := e.manager.NewRef(handle)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		temporaries = append(temporaries, reference)
		return handle, nil
	}
	rows := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		values, x := e.relations.DecodeFact(state.sorts, fact)
		if x != nil {
			return answer, x
		}
		temporaries = append(temporaries, values...)
		equalities := make([]ast.ASTHandle, len(fact))
		for i := range fact {
			variable, y := e.manager.MkVar(uint32(i), state.sorts[i])
			if y != nil {
				return answer, y
			}
			if _, y = retain(variable); y != nil {
				return answer, y
			}
			value, y := e.manager.RefGet(values[i])
			if y != nil {
				return answer, y
			}
			equalities[i], y = e.manager.MkEq(variable, value)
			if y != nil {
				return answer, y
			}
			if _, y = retain(equalities[i]); y != nil {
				return answer, y
			}
		}
		row, x := e.manager.MkAndArray(equalities)
		if x != nil {
			return answer, x
		}
		if row, x = retain(row); x != nil {
			return answer, x
		}
		rows = append(rows, row)
	}
	formula, err := e.manager.MkOrArray(rows)
	if err != nil {
		return answer, err
	}
	return e.manager.NewRef(formula)
}

// Model returns the inductive PDR or Spacer model for the latest unreachable
// query. The caller owns and closes the returned model.
func (e *PropositionalEngine) Model(declaration ast.ASTHandle) (*model.Model, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if !e.queried {
		return nil, ErrPropositionalEngine
	}
	if e.backend == "pdr" && e.pdr != nil {
		return e.pdr.Model(declaration)
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		result, err := e.spacer.Model(e.spacerPredicate(declaration))
		if err != nil {
			return nil, err
		}
		return e.expandSpacerModel(result)
	}
	return nil, ErrPropositionalEngine
}

// Query accepts the zero-arity predicate application used by the original
// propositional engine API. General declaration queries use QueryPredicate.
func (e *PropositionalEngine) Query(query ast.ASTHandle) (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	kind, err := e.manager.GetKind(query)
	if err != nil {
		return lbool.Undef, err
	}
	if kind != ast.ASTApp {
		return lbool.Undef, ErrPropositionalHorn
	}
	arguments, err := e.manager.GetNumArgs(query)
	if err != nil {
		return lbool.Undef, err
	}
	if arguments != 0 {
		return lbool.Undef, ErrPropositionalHorn
	}
	declaration, err := e.manager.GetDecl(query)
	if err != nil {
		return lbool.Undef, err
	}
	return e.QueryPredicate(declaration)
}

func (e *PropositionalEngine) Close() (result error) {
	if e == nil {
		return ErrPropositionalEngine
	}
	if e.closed {
		return nil
	}
	if e.pdr != nil {
		result = errors.Join(result, e.pdr.Close())
		e.pdr = nil
		e.pdrRevision = 0
	}
	if e.spacer != nil {
		result = errors.Join(result, e.spacer.Close())
		e.spacer = nil
		e.spacerRevision = 0
		e.spacerPredicates = nil
		e.spacerSlices = nil
		e.spacerQuantified = nil
		e.spacerInstantiated = nil
		e.spacerBitBlasted = nil
		e.spacerScaled = nil
		e.spacerMagic = nil
	}
	for i := len(e.scopes) - 1; i >= 0; i-- {
		result = errors.Join(result, e.scopes[i].Close())
	}
	e.scopes = nil
	if e.guard != nil {
		result = errors.Join(result, e.guard.Close())
		e.guard = nil
	}
	if e.set != nil {
		result = errors.Join(result, e.set.Close())
		e.set = nil
	}
	if e.rules != nil {
		result = errors.Join(result, e.rules.Close())
		e.rules = nil
	}
	if e.binder != nil {
		result = errors.Join(result, e.binder.Close())
		e.binder = nil
	}
	if e.relations != nil {
		result = errors.Join(result, e.relations.Close())
		e.relations = nil
	}
	result = errors.Join(result, e.manager.DelRef(&e.background))
	e.predicates = nil
	e.predicateOrder = nil
	e.auxiliary = nil
	e.params, e.globals = nil, nil
	e.closed = true
	return result
}
