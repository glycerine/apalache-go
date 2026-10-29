package apalache

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

const maxExpandedIntegerRange = 4096

const AssignmentManualLink = "https://apalache-mc.org/docs/apalache/principles/assignments.html"

type assignmentPartialState struct {
	unassigned map[string]struct{}
	strategy   AssignmentStrategy
}

// SMTFreeSymbolicTransitionExtractor finds an assignment-before-use strategy
// without consulting an SMT solver.
type SMTFreeSymbolicTransitionExtractor struct {
	tracker TransformationTracker
	sources *SourceStore
}

func NewSMTFreeSymbolicTransitionExtractor(tracker TransformationTracker, sources *SourceStore) *SMTFreeSymbolicTransitionExtractor {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &SMTFreeSymbolicTransitionExtractor{tracker: tracker, sources: sources}
}

func (extractor *SMTFreeSymbolicTransitionExtractor) GetStrategy(
	variables map[string]struct{},
	action Expr,
	operators map[string]*OperDecl,
) (AssignmentStrategy, error) {
	if action == nil {
		return nil, &AssignmentError{Message: "cannot extract assignments from a nil expression"}
	}
	state := assignmentPartialState{unassigned: cloneMap(variables), strategy: nil}
	final, err := extractor.strategy(state, cloneMap(operators), action)
	if err != nil {
		return nil, err
	}
	if len(final.unassigned) != 0 {
		return nil, &AssignmentError{Message: "No assignments found for: " + strings.Join(sortedStringSet(final.unassigned), ", ")}
	}
	return append(AssignmentStrategy(nil), final.strategy...), nil
}

// Extract finds a valid assignment strategy and slices the action into its
// symbolic transitions. Specifications without state variables need no
// strategy or slicing.
func (extractor *SMTFreeSymbolicTransitionExtractor) Extract(
	variables map[string]struct{},
	action Expr,
	operators map[string]*OperDecl,
) ([]SymbolicTransition, error) {
	if action == nil {
		return nil, &AssignmentError{Message: "cannot extract transitions from a nil expression"}
	}
	if len(variables) == 0 {
		return []SymbolicTransition{{Expression: action}}, nil
	}
	strategy, err := extractor.GetStrategy(variables, action, operators)
	if err != nil {
		return nil, err
	}
	return NewSymbolicTransitionGenerator(extractor.tracker).Generate(action, strategy)
}

// expandFiniteQuantifiers exposes choices over explicit finite sets before
// transition slicing. In particular, an existential whose body assigns a
// primed variable must produce one symbolic transition per possible value;
// otherwise deadlock checking can mistake "some target is enabled" for
// "every target is enabled" on a symbolic path.
func expandFiniteQuantifiers(expression Expr, assignmentVariables map[string]struct{}, tracker TransformationTracker) Expr {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return RewriteExpressions(tracker, func(current Expr) (Expr, bool) {
		quantifier, ok := current.(*OperExpr)
		if !ok || len(quantifier.Arguments()) != 3 {
			return nil, false
		}
		name := quantifier.Operator().Name()
		if name != "EXISTS3" {
			return nil, false
		}
		arguments := quantifier.Arguments()
		binder, ok := arguments[0].(*NameExpr)
		if !ok {
			return nil, false
		}
		if !quantifierAssignsFromBinder(arguments[2], binder.Name(), assignmentVariables) {
			return nil, false
		}
		members, ok := finiteQuantifierMembers(arguments[1])
		if !ok {
			return nil, false
		}
		if len(members) == 0 {
			return NewValueExpr(BoolValue(false), quantifier.Tag()), true
		}
		branches := make([]Expr, len(members))
		for index, member := range members {
			branches[index] = substituteBoundName(arguments[2], binder.Name(), member, tracker)
		}
		if len(branches) == 1 {
			return branches[0], true
		}
		return MustOperExpr(Op("OR"), quantifier.Tag(), branches...), true
	})(expression)
}

func finiteQuantifierMembers(domain Expr) ([]Expr, bool) {
	set, ok := domain.(*OperExpr)
	if !ok {
		return nil, false
	}
	if set.Operator().Name() == "SET_ENUM" {
		return set.Arguments(), true
	}
	arguments := set.Arguments()
	if set.Operator().Name() != "INT_RANGE" || len(arguments) != 2 {
		return nil, false
	}
	lowerExpr, lowerOK := arguments[0].(*ValueExpr)
	upperExpr, upperOK := arguments[1].(*ValueExpr)
	if !lowerOK || !upperOK {
		return nil, false
	}
	lower, lowerOK := lowerExpr.Value().(IntValue)
	upper, upperOK := upperExpr.Value().(IntValue)
	if !lowerOK || !upperOK {
		return nil, false
	}
	first, last := lower.BigInt(), upper.BigInt()
	if first.Cmp(last) > 0 {
		return nil, true
	}
	width := new(big.Int).Sub(last, first)
	width.Add(width, big.NewInt(1))
	if !width.IsInt64() || width.Int64() > maxExpandedIntegerRange {
		return nil, false
	}
	members := make([]Expr, 0, int(width.Int64()))
	for value := new(big.Int).Set(first); value.Cmp(last) <= 0; value.Add(value, big.NewInt(1)) {
		members = append(members, NewValueExpr(NewIntValue(value), Typed(IntT1)))
	}
	return members, true
}

func quantifierAssignsFromBinder(expression Expr, binder string, variables map[string]struct{}) bool {
	operation, ok := expression.(*OperExpr)
	if ok {
		arguments := operation.Arguments()
		if len(arguments) == 2 && (operation.Operator().Name() == "EQ" || operation.Operator().Name() == "Apalache!:=") &&
			isPrimedName(arguments[0]) {
			name := arguments[0].(*OperExpr).Arguments()[0].(*NameExpr).Name()
			if _, eligible := variables[name]; eligible && expressionContainsName(arguments[1], binder) {
				return true
			}
		}
		for _, argument := range arguments {
			if quantifierAssignsFromBinder(argument, binder, variables) {
				return true
			}
		}
	}
	if let, ok := expression.(*LetInExpr); ok {
		if quantifierAssignsFromBinder(let.Body(), binder, variables) {
			return true
		}
		for _, declaration := range let.Declarations() {
			if quantifierAssignsFromBinder(declaration.Body(), binder, variables) {
				return true
			}
		}
	}
	return false
}

func expressionContainsName(expression Expr, wanted string) bool {
	if name, ok := expression.(*NameExpr); ok {
		return name.Name() == wanted
	}
	switch typed := expression.(type) {
	case *OperExpr:
		for _, argument := range typed.Arguments() {
			if expressionContainsName(argument, wanted) {
				return true
			}
		}
	case *LetInExpr:
		if expressionContainsName(typed.Body(), wanted) {
			return true
		}
		for _, declaration := range typed.Declarations() {
			if expressionContainsName(declaration.Body(), wanted) {
				return true
			}
		}
	}
	return false
}

func substituteBoundName(expression Expr, name string, replacement Expr, tracker TransformationTracker) Expr {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var substitute func(Expr) Expr
	substitute = func(current Expr) Expr {
		if named, ok := current.(*NameExpr); ok && named.Name() == name {
			return replacement
		}
		switch typed := current.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			isQuantifier := typed.Operator().Name() == "EXISTS3" || typed.Operator().Name() == "FORALL3"
			for index := range arguments {
				if isQuantifier && index == 0 {
					continue
				}
				// A nested binder does not scope over its own domain, but it
				// shadows the outer name in its predicate.
				if isQuantifier && index == 2 {
					binder, _ := arguments[0].(*NameExpr)
					if binder != nil && binder.Name() == name {
						continue
					}
				}
				arguments[index] = substitute(arguments[index])
			}
			result := MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			return tracker.TrackExpression(func(Expr) Expr { return result })(current)
		case *LetInExpr:
			declarations := typed.Declarations()
			for index, declaration := range declarations {
				copy := declaration.copyWithTag(declaration.Tag())
				shadowed := false
				for _, parameter := range declaration.FormalParams() {
					if parameter.Name == name {
						shadowed = true
						break
					}
				}
				if !shadowed {
					copy.SetBody(substitute(declaration.Body()))
				}
				declarations[index] = copy
			}
			result := NewLetInExpr(substitute(typed.Body()), typed.Tag(), declarations...)
			return tracker.TrackExpression(func(Expr) Expr { return result })(current)
		default:
			return current
		}
	}
	return substitute(expression)
}

func (extractor *SMTFreeSymbolicTransitionExtractor) strategy(
	state assignmentPartialState,
	operators map[string]*OperDecl,
	expression Expr,
) (assignmentPartialState, error) {
	op, operation := expression.(*OperExpr)
	if operation {
		arguments := op.Arguments()
		name := op.Operator().Name()
		if (name == "EQ" || name == "Apalache!:=") && isPrimedName(arguments[0]) {
			if err := extractor.checkAssignmentFree(state.unassigned, arguments[1]); err != nil {
				return state, err
			}
			variable := arguments[0].(*OperExpr).Arguments()[0].(*NameExpr).Name()
			if _, unassigned := state.unassigned[variable]; unassigned {
				remaining := cloneMap(state.unassigned)
				delete(remaining, variable)
				strategy := append(append(AssignmentStrategy(nil), state.strategy...), op.ID())
				return assignmentPartialState{unassigned: remaining, strategy: strategy}, nil
			}
			if name == "Apalache!:=" {
				return state, &AssignmentError{Message: fmt.Sprintf("%s: Manual assignment is spurious, %s is already assigned! See %s", extractor.location(op), variable, AssignmentManualLink)}
			}
			return state, nil
		}
		switch name {
		case "AND":
			current := state
			for _, argument := range arguments {
				var err error
				current, err = extractor.strategy(current, operators, argument)
				if err != nil {
					return state, err
				}
			}
			return current, nil
		case "OR":
			return extractor.branches(state, operators, arguments)
		case "EXISTS3":
			if err := extractor.checkAssignmentFree(state.unassigned, arguments[1]); err != nil {
				return state, err
			}
			return extractor.strategy(state, operators, arguments[2])
		case "LABEL":
			return extractor.strategy(state, operators, arguments[0])
		case "IF_THEN_ELSE":
			if err := extractor.checkAssignmentFree(state.unassigned, arguments[0]); err != nil {
				return state, err
			}
			return extractor.branches(state, operators, arguments[1:])
		case "OPER_APP":
			if applied, named := arguments[0].(*NameExpr); named {
				if declaration, found := operators[applied.Name()]; found {
					if declaration.IsRecursive() {
						if err := extractor.checkAssignmentFree(state.unassigned, declaration.Body()); err != nil {
							return state, err
						}
						return state, nil
					}
					return extractor.strategy(state, operators, declaration.Body())
				}
			}
			return state, nil
		default:
			for _, argument := range arguments {
				if err := extractor.checkAssignmentFree(state.unassigned, argument); err != nil {
					return state, err
				}
			}
			return state, nil
		}
	}
	if let, ok := expression.(*LetInExpr); ok {
		inside := cloneMap(operators)
		for _, declaration := range let.Declarations() {
			inside[declaration.Name()] = declaration
		}
		return extractor.strategy(state, inside, let.Body())
	}
	return state, nil
}

func (extractor *SMTFreeSymbolicTransitionExtractor) branches(
	state assignmentPartialState,
	operators map[string]*OperDecl,
	branches []Expr,
) (assignmentPartialState, error) {
	children := make([]assignmentPartialState, len(branches))
	for index, branch := range branches {
		child, err := extractor.strategy(state, operators, branch)
		if err != nil {
			return state, err
		}
		children[index] = child
	}
	unassignedEverywhere := cloneMap(state.unassigned)
	unassignedSomewhere := make(map[string]struct{})
	for _, child := range children {
		for variable := range unassignedEverywhere {
			if _, found := child.unassigned[variable]; !found {
				delete(unassignedEverywhere, variable)
			}
		}
		for variable := range child.unassigned {
			unassignedSomewhere[variable] = struct{}{}
		}
	}
	imbalanced := make(map[string]struct{})
	for variable := range unassignedSomewhere {
		if _, everywhere := unassignedEverywhere[variable]; !everywhere {
			imbalanced[variable] = struct{}{}
		}
	}
	if len(imbalanced) != 0 {
		problems := make([]string, 0)
		for index, child := range children {
			missing := make(map[string]struct{})
			for variable := range child.unassigned {
				if _, relevant := imbalanced[variable]; relevant {
					missing[variable] = struct{}{}
				}
			}
			if len(missing) != 0 {
				problems = append(problems, fmt.Sprintf("%s: Missing assignments to: %s", extractor.location(branches[index]), strings.Join(sortedStringSet(missing), ", ")))
			}
		}
		return state, &AssignmentError{Message: strings.Join(problems, "\n")}
	}
	unified := append(AssignmentStrategy(nil), state.strategy...)
	seen := make(map[UID]struct{}, len(unified))
	for _, id := range unified {
		seen[id] = struct{}{}
	}
	for _, child := range children {
		for _, id := range child.strategy {
			if _, found := seen[id]; !found {
				seen[id] = struct{}{}
				unified = append(unified, id)
			}
		}
	}
	return assignmentPartialState{unassigned: unassignedEverywhere, strategy: unified}, nil
}

func (extractor *SMTFreeSymbolicTransitionExtractor) checkAssignmentFree(unassigned map[string]struct{}, expression Expr) error {
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		if typed.Operator().Name() == "Apalache!:=" && isPrimedName(arguments[0]) {
			return &AssignmentError{Message: fmt.Sprintf("%s: Illegal assignment inside an assignment-free expression. See %s", extractor.location(typed), AssignmentManualLink)}
		}
		if typed.Operator().Name() == "PRIME" {
			if name, named := arguments[0].(*NameExpr); named {
				if _, missing := unassigned[name.Name()]; missing {
					return &AssignmentError{Message: fmt.Sprintf("%s: %s' is used before it is assigned. See %s", extractor.location(typed), name.Name(), AssignmentManualLink)}
				}
			}
		}
		for _, argument := range arguments {
			if err := extractor.checkAssignmentFree(unassigned, argument); err != nil {
				return err
			}
		}
	case *LetInExpr:
		for _, declaration := range typed.Declarations() {
			if err := extractor.checkAssignmentFree(unassigned, declaration.Body()); err != nil {
				return err
			}
		}
		return extractor.checkAssignmentFree(unassigned, typed.Body())
	}
	return nil
}

func (extractor *SMTFreeSymbolicTransitionExtractor) location(expression Expr) string {
	if extractor.sources != nil {
		if location, found := extractor.sources.Find(expression.ID()); found {
			return location.String()
		}
	}
	return "<[UNKNOWN]>"
}

func sortedStringSet(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func OperatorBodyMap(declarations ...*OperDecl) map[string]*OperDecl {
	result := make(map[string]*OperDecl, len(declarations))
	for _, declaration := range declarations {
		if declaration != nil {
			result[declaration.Name()] = declaration
		}
	}
	return result
}
