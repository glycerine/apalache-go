package apalache

import (
	"fmt"
	"strings"
)

type VCGenerator struct{ tracker TransformationTracker }

func NewVCGenerator(tracker TransformationTracker) *VCGenerator {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &VCGenerator{tracker: tracker}
}

// GenerateVerificationConditions is the Go pass boundary corresponding to
// VCGenPassImpl. Invariants are expanded in configuration order, followed by
// the optional normalized state view.
func GenerateVerificationConditions(module *Module, predicates DerivedPredicates, tracker TransformationTracker) (*Module, error) {
	result, _, err := generateVerificationConditions(module, predicates, tracker)
	return result, err
}

type verificationConditionSources struct {
	state  []string
	action []string
	trace  []string
}

func generateVerificationConditions(module *Module, predicates DerivedPredicates, tracker TransformationTracker) (*Module, verificationConditionSources, error) {
	if module == nil {
		return nil, verificationConditionSources{}, fmt.Errorf("cannot generate verification conditions for a nil module")
	}
	if !module.HasProperty(PropertyInlined) {
		return nil, verificationConditionSources{}, fmt.Errorf("VC generation requires an inlined module")
	}
	generator := NewVCGenerator(tracker)
	current := module
	sources := verificationConditionSources{}
	var err error
	for _, invariant := range predicates.Invariants {
		declarationCount := len(current.Declarations())
		current, err = generator.GenerateInvariant(current, invariant)
		if err != nil {
			return nil, verificationConditionSources{}, err
		}
		for _, declaration := range current.Declarations()[declarationCount:] {
			switch {
			case strings.HasPrefix(declaration.Name(), NormalizedVCInvariantPrefix):
				sources.state = append(sources.state, invariant)
			case strings.HasPrefix(declaration.Name(), NormalizedVCActionPrefix):
				sources.action = append(sources.action, invariant)
			case strings.HasPrefix(declaration.Name(), NormalizedVCTracePrefix):
				sources.trace = append(sources.trace, invariant)
			}
		}
	}
	if predicates.View != "" {
		current, err = generator.GenerateView(current, predicates.View)
		if err != nil {
			return nil, verificationConditionSources{}, err
		}
	}
	current.AddProperty(PropertyVCGenerated)
	return current, sources, nil
}

func (generator *VCGenerator) moduleWith(module *Module, declarations ...Decl) *Module {
	result := NewModule(module.Name(), append(module.Declarations(), declarations...))
	result.copyMetadataFrom(module)
	return result
}

func (generator *VCGenerator) GenerateInvariant(module *Module, name string) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot generate verification conditions for a nil module")
	}
	var declaration Decl
	for _, candidate := range module.Declarations() {
		if candidate.Name() == name {
			declaration = candidate
			break
		}
	}
	if declaration == nil {
		return nil, NewTlaInputError("Invariant candidate " + name + " not found")
	}
	operator, ok := declaration.(*OperDecl)
	if !ok {
		return nil, NewTlaInputError(fmt.Sprintf("Expected a nullary operator %s, found %T", name, declaration))
	}
	parameters := operator.FormalParams()
	if len(parameters) == 0 {
		level := NewLevelFinder(module).LevelOfDeclaration(operator)
		if level == TemporalLevel {
			return nil, NewTlaInputError("Expected a state invariant or an action invariant in "+name+
				", found a temporal property (did you mean to use --temporal?)", operator.Body().ID())
		}
		conditions, err := generator.introduceConditions(level, operator.Body())
		if err != nil {
			return nil, err
		}
		return generator.moduleWith(module, conditions...), nil
	}
	if len(parameters) == 1 && parameters[0].Arity == 0 {
		if NewLevelFinder(module).LevelOfDeclaration(operator) != ConstantLevel {
			return nil, NewTlaInputError("Trace invariant "+name+
				" should not refer to state variables or use action/temporal operators", operator.Body().ID())
		}
		if _, err := validateTraceInvariantType(operator, module.VarDeclarations()); err != nil {
			return nil, NewMalformedTlaError("Expected the trace invariant "+operator.Name()+
				" to be a predicate of a sequence of records over the names of state variables", operator.Body())
		}
		copy := NewDeepCopier(generator.tracker)
		positiveBody := copy.Expression(operator.Body())
		negativeCopy := copy.Expression(operator.Body())
		negativeBody := MustOperExpr(Op("NOT"), Typed(BoolT1), negativeCopy)
		positive := NewOperDecl(NormalizedVCTracePrefix+"0", parameters, positiveBody, operator.Tag())
		negative := NewOperDecl(NormalizedVCNotTracePrefix+"0", parameters, negativeBody, operator.Tag())
		generator.tracker.Hold(operator.Body(), positiveBody)
		generator.tracker.Hold(operator.Body(), negativeBody)
		return generator.moduleWith(module, positive, negative), nil
	}
	return nil, NewTlaInputError(fmt.Sprintf(
		"Expected a state/action invariant %s (0 parameters) or a trace invariant (1 parameter), found %d parameters",
		name, len(parameters)), operator.Body().ID())
}

func (generator *VCGenerator) GenerateView(module *Module, name string) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot generate a view for a nil module")
	}
	for _, declaration := range module.Declarations() {
		if declaration.Name() != name {
			continue
		}
		operator, ok := declaration.(*OperDecl)
		if !ok {
			return nil, NewMalformedTlaError(fmt.Sprintf("Expected state view %s to be an operator, found: %T", name, declaration), NullExpr())
		}
		if len(operator.FormalParams()) != 0 {
			return nil, NewMalformedTlaError(fmt.Sprintf("Expected state view %s to have no parameters, found: %d parameters", name, len(operator.FormalParams())), operator.Body())
		}
		body := NewDeepCopier(generator.tracker).Expression(operator.Body())
		return generator.moduleWith(module, NewOperDecl(NormalizedVCView, nil, body, operator.Tag())), nil
	}
	return nil, NewMalformedTlaError("State view "+name+" not found", NullExpr())
}

func (generator *VCGenerator) introduceConditions(level Level, invariant Expr) ([]Decl, error) {
	fragments := generator.splitInvariant(nil, invariant)
	result := make([]Decl, 0, len(fragments)*2)
	for index, fragment := range fragments {
		normalized := RewriteExpressions(generator.tracker, func(expression Expr) (Expr, bool) {
			operation, ok := expression.(*OperExpr)
			if !ok || operation.Operator().Name() != "Apalache!:=" {
				return nil, false
			}
			arguments := operation.Arguments()
			return MustOperExpr(Op("EQ"), operation.Tag(), arguments...), true
		})(fragment)
		copy := NewDeepCopier(generator.tracker).Expression(normalized)
		positivePrefix, negativePrefix := NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix
		if level == ActionLevel {
			positivePrefix, negativePrefix = NormalizedVCActionPrefix, NormalizedVCNotActionPrefix
		}
		positive := NewOperDecl(fmt.Sprintf("%s%d", positivePrefix, index), nil, normalized, invariant.Tag())
		negativeBody := MustOperExpr(Op("NOT"), Typed(BoolT1), copy)
		negative := NewOperDecl(fmt.Sprintf("%s%d", negativePrefix, index), nil, negativeBody, invariant.Tag())
		generator.tracker.Hold(fragment, positive.Body())
		generator.tracker.Hold(fragment, negative.Body())
		result = append(result, positive, negative)
	}
	return result, nil
}

func (generator *VCGenerator) splitInvariant(universals []Pair[*NameExpr, Expr], invariant Expr) []Expr {
	if operation, ok := invariant.(*OperExpr); ok {
		arguments := operation.Arguments()
		switch operation.Operator().Name() {
		case "FORALL3":
			if len(arguments) == 3 {
				if name, ok := arguments[0].(*NameExpr); ok {
					return generator.splitInvariant(append([]Pair[*NameExpr, Expr]{{First: name, Second: arguments[1]}}, universals...), arguments[2])
				}
			}
		case "AND":
			var result []Expr
			for _, argument := range arguments {
				result = append(result, generator.splitInvariant(universals, argument)...)
			}
			return result
		case "LABEL":
			if len(arguments) >= 2 {
				pieces := generator.splitInvariant(universals, arguments[0])
				for index, piece := range pieces {
					pieces[index] = MustOperExpr(Op("LABEL"), piece.Tag(), append([]Expr{piece}, arguments[1:]...)...)
					generator.tracker.Hold(piece, pieces[index])
				}
				return pieces
			}
		}
	}
	result := invariant
	for _, binding := range universals {
		result = MustOperExpr(Op("FORALL3"), Typed(BoolT1), binding.First, binding.Second, result)
		generator.tracker.Hold(invariant, result)
	}
	return []Expr{result}
}
