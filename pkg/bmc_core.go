package apalache

import "fmt"

const (
	MaxProductSize          = 1_000_000
	PowSetMaxBaseSize       = 20
	RewritingRecursionLimit = 10_000
)

// InvariantKind is the closed state/action invariant classification.
type InvariantKind uint8

const (
	StateInvariant InvariantKind = iota
	ActionInvariant
)

func (kind InvariantKind) String() string {
	switch kind {
	case StateInvariant:
		return "state"
	case ActionInvariant:
		return "action"
	default:
		return fmt.Sprintf("InvariantKind(%d)", kind)
	}
}

// ExpressionVerificationCondition pairs an invariant with its negation.
type ExpressionVerificationCondition struct {
	Invariant Expr
	Negation  Expr
}

// TraceVerificationCondition pairs a trace-invariant declaration with its
// negation.
type TraceVerificationCondition struct {
	Invariant *OperDecl
	Negation  *OperDecl
}

// CheckerVerificationConditions is the Go counterpart of CheckerInputVC.
// Its accessors copy slices to retain Scala's immutable-List behavior.
type CheckerVerificationConditions struct {
	state  []ExpressionVerificationCondition
	action []ExpressionVerificationCondition
	trace  []TraceVerificationCondition
	view   Expr
}

func NewCheckerVerificationConditions(
	state []ExpressionVerificationCondition,
	action []ExpressionVerificationCondition,
	trace []TraceVerificationCondition,
	view Expr,
) CheckerVerificationConditions {
	return CheckerVerificationConditions{
		state:  append([]ExpressionVerificationCondition(nil), state...),
		action: append([]ExpressionVerificationCondition(nil), action...),
		trace:  append([]TraceVerificationCondition(nil), trace...),
		view:   view,
	}
}

func (conditions CheckerVerificationConditions) StateInvariantsAndNegations() []ExpressionVerificationCondition {
	return append([]ExpressionVerificationCondition(nil), conditions.state...)
}

func (conditions CheckerVerificationConditions) ActionInvariantsAndNegations() []ExpressionVerificationCondition {
	return append([]ExpressionVerificationCondition(nil), conditions.action...)
}

func (conditions CheckerVerificationConditions) TraceInvariantsAndNegations() []TraceVerificationCondition {
	return append([]TraceVerificationCondition(nil), conditions.trace...)
}

func (conditions CheckerVerificationConditions) StateView() (Expr, bool) {
	return conditions.view, conditions.view != nil
}

// CheckerInput is the normalized transition/checker boundary produced by the
// transition pass.
type CheckerInput struct {
	rootModule      *Module
	initTransitions []Expr
	nextTransitions []Expr
	constInitPrimed Expr
	conditions      CheckerVerificationConditions
	persistentDecls map[string]*OperDecl
}

func NewCheckerInput(
	rootModule *Module,
	initTransitions []Expr,
	nextTransitions []Expr,
	constInitPrimed Expr,
	conditions CheckerVerificationConditions,
	persistentDecls map[string]*OperDecl,
) *CheckerInput {
	return &CheckerInput{
		rootModule:      rootModule,
		initTransitions: append([]Expr(nil), initTransitions...),
		nextTransitions: append([]Expr(nil), nextTransitions...),
		constInitPrimed: constInitPrimed,
		conditions:      conditions,
		persistentDecls: cloneMap(persistentDecls),
	}
}

func (input *CheckerInput) RootModule() *Module { return input.rootModule }
func (input *CheckerInput) InitTransitions() []Expr {
	return append([]Expr(nil), input.initTransitions...)
}
func (input *CheckerInput) NextTransitions() []Expr {
	return append([]Expr(nil), input.nextTransitions...)
}
func (input *CheckerInput) ConstInitPrimed() (Expr, bool) {
	return input.constInitPrimed, input.constInitPrimed != nil
}
func (input *CheckerInput) VerificationConditions() CheckerVerificationConditions {
	return input.conditions
}
func (input *CheckerInput) PersistentDeclarations() map[string]*OperDecl {
	return cloneMap(input.persistentDecls)
}

// ReTLACombinedPredicate applies monotype validation before the ReTLA
// fragment check and preserves both predicates' ordered failures.
type ReTLACombinedPredicate struct{}

func (ReTLACombinedPredicate) LanguagePredicateName() string { return "ReTLACombinedPredicate" }

func (ReTLACombinedPredicate) CheckExpression(expression Expr) PredicateResult {
	return (MonotypeLanguagePredicate{}).CheckExpression(expression).
		And((ReTLALanguagePredicate{}).CheckExpression(expression))
}

func (ReTLACombinedPredicate) CheckModule(module *Module) PredicateResult {
	return (MonotypeLanguagePredicate{}).CheckModule(module).
		And((ReTLALanguagePredicate{}).CheckModule(module))
}
