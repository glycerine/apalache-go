package apalache

import "fmt"

const (
	TemporalTableauPrefix  = "__temporal_"
	TemporalUnrollSuffix   = "_unroll"
	TemporalLookbackSuffix = "_prev"
)

type TemporalPredicateExpressions struct {
	Init   []Expr
	Next   []Expr
	LoopOK []Expr
}

func (predicates TemporalPredicateExpressions) Append(other TemporalPredicateExpressions) TemporalPredicateExpressions {
	return TemporalPredicateExpressions{
		Init:   append(append([]Expr(nil), predicates.Init...), other.Init...),
		Next:   append(append([]Expr(nil), predicates.Next...), other.Next...),
		LoopOK: append(append([]Expr(nil), predicates.LoopOK...), other.LoopOK...),
	}
}

type TemporalSyntaxEncoding struct {
	Variables  []*VarDecl
	Predicates TemporalPredicateExpressions
	Expression Expr
}

type temporalFormulaEncoding struct {
	variables  []*VarDecl
	predicates TemporalPredicateExpressions
	initial    *VarDecl
}

// TemporalTableauEncoder ports the lasso tableau construction of the Scala
// TableauEncoder for Boolean structure and the [] and <> operators.
type TemporalTableauEncoder struct {
	module  *Module
	names   *UniqueNameGenerator
	loop    *TemporalLoopEncoder
	tracker TransformationTracker
	levels  *LevelFinder
}

func NewTemporalTableauEncoder(
	module *Module,
	names *UniqueNameGenerator,
	loop *TemporalLoopEncoder,
	tracker TransformationTracker,
) (*TemporalTableauEncoder, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot create a temporal tableau encoder without a module")
	}
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	if loop == nil {
		loop = NewTemporalLoopEncoder(tracker)
	}
	return &TemporalTableauEncoder{module: module, names: names, loop: loop, tracker: tracker, levels: NewLevelFinder(module)}, nil
}

func temporalBooleanSetMembership(expression Expr) Expr {
	setType, _ := NewSetType(BoolT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), temporalBool(true), temporalBool(false))
	return MustOperExpr(Op("SET_IN"), Typed(BoolT1), expression, set)
}

func temporalEquivalence(left, right Expr) Expr {
	return MustOperExpr(Op("EQUIV"), Typed(BoolT1), left, right)
}

func temporalNot(expression Expr) Expr {
	return MustOperExpr(Op("NOT"), Typed(BoolT1), expression)
}

func temporalOr(expressions ...Expr) Expr {
	return MustOperExpr(Op("OR"), Typed(BoolT1), expressions...)
}

func rewriteTemporalAssignments(expression Expr) Expr {
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		for index, argument := range arguments {
			arguments[index] = rewriteTemporalAssignments(argument)
		}
		operator := typed.Operator()
		if operator.Name() == "Apalache!"+":=" {
			operator = Op("EQ")
		}
		return MustOperExpr(operator, typed.Tag(), arguments...)
	case *LetInExpr:
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			copy := declaration.copyWithTag(declaration.Tag())
			copy.SetBody(rewriteTemporalAssignments(declaration.Body()))
			declarations[index] = copy
		}
		return NewLetInExpr(rewriteTemporalAssignments(typed.Body()), typed.Tag(), declarations...)
	default:
		return expression
	}
}

func (encoder *TemporalTableauEncoder) genericPredicates(node, saved *VarDecl) TemporalPredicateExpressions {
	nodeExpression := temporalName(node)
	savedExpression := temporalName(saved)
	return TemporalPredicateExpressions{
		Init: []Expr{
			temporalBooleanSetMembership(nodeExpression),
			MustOperExpr(Op("EQ"), Typed(BoolT1), savedExpression, nodeExpression),
		},
		Next: []Expr{
			temporalBooleanSetMembership(temporalPrime(nodeExpression)),
			encoder.loop.SavedVariableUpdate(node, saved),
		},
		LoopOK: []Expr{MustOperExpr(Op("EQ"), Typed(BoolT1), nodeExpression, savedExpression)},
	}
}

func (encoder *TemporalTableauEncoder) EncodeSyntaxTree(expression Expr) (TemporalSyntaxEncoding, error) {
	if encoder.levels.LevelOfExpression(expression) != TemporalLevel {
		return TemporalSyntaxEncoding{Expression: expression}, nil
	}
	operation, ok := expression.(*OperExpr)
	if !ok {
		return TemporalSyntaxEncoding{}, fmt.Errorf("temporal-level expression %T must be an operator application after inlining", expression)
	}
	node := NewVarDecl(TemporalTableauPrefix+encoder.names.NewName(), Typed(BoolT1))
	saved := encoder.loop.SavedVariable(node)
	nodeExpression := temporalName(node)
	predicates := encoder.genericPredicates(node, saved)
	variables := []*VarDecl{node, saved}
	arguments := operation.Arguments()
	encodedArguments := make([]Expr, len(arguments))
	for index, argument := range arguments {
		encoded, err := encoder.EncodeSyntaxTree(argument)
		if err != nil {
			return TemporalSyntaxEncoding{}, err
		}
		variables = append(variables, encoded.Variables...)
		predicates = predicates.Append(encoded.Predicates)
		encodedArguments[index] = encoded.Expression
	}

	switch operation.Operator().Name() {
	case "GLOBALLY", "EVENTUALLY":
		if len(encodedArguments) != 1 {
			return TemporalSyntaxEncoding{}, fmt.Errorf("temporal operator %s has %d arguments", operation.Operator().Name(), len(encodedArguments))
		}
		isGlobally := operation.Operator().Name() == "GLOBALLY"
		auxiliary := NewVarDecl(node.Name()+TemporalUnrollSuffix, Typed(BoolT1))
		lookback := NewVarDecl(auxiliary.Name()+TemporalLookbackSuffix, Typed(BoolT1))
		variables = append([]*VarDecl{node, saved, auxiliary, lookback}, variables[2:]...)
		auxiliaryExpression := temporalName(auxiliary)
		lookbackExpression := temporalName(lookback)
		initialValue := temporalBool(isGlobally)
		inLoopPrime := temporalPrime(NewNameExpr(TemporalInLoopName, Typed(BoolT1)))
		var recurrence, unroll Expr
		if isGlobally {
			recurrence = temporalAnd(encodedArguments[0], temporalPrime(nodeExpression))
			unroll = temporalAnd(lookbackExpression, temporalOr(temporalNot(inLoopPrime), encodedArguments[0]))
		} else {
			recurrence = temporalOr(encodedArguments[0], temporalPrime(nodeExpression))
			unroll = temporalOr(lookbackExpression, temporalAnd(inLoopPrime, encodedArguments[0]))
		}
		predicates = predicates.Append(TemporalPredicateExpressions{
			Init: []Expr{
				MustOperExpr(Op("EQ"), Typed(BoolT1), auxiliaryExpression, initialValue),
				MustOperExpr(Op("EQ"), Typed(BoolT1), lookbackExpression, initialValue),
			},
			Next: []Expr{
				temporalBooleanSetMembership(temporalPrime(auxiliaryExpression)),
				temporalEquivalence(nodeExpression, recurrence),
				temporalEquivalence(auxiliaryExpression, unroll),
				MustOperExpr(Op("EQ"), Typed(BoolT1), temporalPrime(lookbackExpression), auxiliaryExpression),
			},
			LoopOK: []Expr{
				func() Expr {
					if isGlobally {
						return MustOperExpr(Op("IMPLIES"), Typed(BoolT1), auxiliaryExpression, nodeExpression)
					}
					return MustOperExpr(Op("IMPLIES"), Typed(BoolT1), nodeExpression, auxiliaryExpression)
				}(),
				MustOperExpr(Op("EQ"), Typed(BoolT1), lookbackExpression, auxiliaryExpression),
			},
		})
	case "LEADS_TO":
		return TemporalSyntaxEncoding{}, fmt.Errorf("temporal operator ~> must be removed by desugaring before tableau encoding")
	case "GUARANTEES":
		return TemporalSyntaxEncoding{}, fmt.Errorf("temporal guarantees is not supported")
	case "WEAK_FAIRNESS", "STRONG_FAIRNESS":
		return TemporalSyntaxEncoding{}, fmt.Errorf("temporal fairness is not supported")
	case "TEMPORAL_EXISTS", "TEMPORAL_FORALL":
		return TemporalSyntaxEncoding{}, fmt.Errorf("temporal quantifiers are not supported")
	default:
		rebuilt := MustOperExpr(operation.Operator(), operation.Tag(), encodedArguments...)
		predicates = predicates.Append(TemporalPredicateExpressions{
			Init: []Expr{MustOperExpr(Op("EQ"), Typed(BoolT1), nodeExpression, rebuilt)},
			Next: []Expr{MustOperExpr(Op("EQ"), Typed(BoolT1), temporalPrime(nodeExpression), rebuilt)},
		})
	}
	return TemporalSyntaxEncoding{Variables: variables, Predicates: predicates, Expression: nodeExpression}, nil
}

func (encoder *TemporalTableauEncoder) encodeFormula(formula *OperDecl) (temporalFormulaEncoding, error) {
	if formula == nil || len(formula.FormalParams()) != 0 {
		return temporalFormulaEncoding{}, fmt.Errorf("temporal property must be a nullary operator")
	}
	encoded, err := encoder.EncodeSyntaxTree(rewriteTemporalAssignments(formula.Body()))
	if err != nil {
		return temporalFormulaEncoding{}, fmt.Errorf("encode temporal property %s: %w", formula.Name(), err)
	}
	initial := NewVarDecl("__"+formula.Name()+"_init", Typed(BoolT1))
	initialExpression := temporalName(initial)
	encoded.Variables = append(encoded.Variables, initial)
	encoded.Predicates = encoded.Predicates.Append(TemporalPredicateExpressions{
		Init: []Expr{MustOperExpr(Op("EQ"), Typed(BoolT1), initialExpression, encoded.Expression)},
		Next: []Expr{MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), initialExpression)},
	})
	return temporalFormulaEncoding{variables: encoded.Variables, predicates: encoded.Predicates, initial: initial}, nil
}

func (encoder *TemporalTableauEncoder) TemporalsToInvariants(
	loop TemporalLoopEncoding,
	formulas ...*OperDecl,
) (*Module, error) {
	allVariables := make([]*VarDecl, 0)
	allPredicates := TemporalPredicateExpressions{}
	initials := make([]*VarDecl, len(formulas))
	formulaNames := make(map[string]struct{}, len(formulas))
	for index, formula := range formulas {
		encoded, err := encoder.encodeFormula(formula)
		if err != nil {
			return nil, err
		}
		allVariables = append(allVariables, encoded.variables...)
		allPredicates = allPredicates.Append(encoded.predicates)
		initials[index] = encoded.initial
		formulaNames[formula.Name()] = struct{}{}
	}
	newInit := encoder.extend(loop.Init, temporalAnd(allPredicates.Init...))
	newNext := encoder.extend(loop.Next, temporalAnd(allPredicates.Next...))
	newLoopOK := encoder.extend(loop.LoopOK, temporalAnd(allPredicates.LoopOK...))
	loopApplication := MustOperExpr(Op("OPER_APP"), Typed(BoolT1),
		NewNameExpr(newLoopOK.Name(), Typed(mustOperator(nil, BoolT1))))
	newFormulas := make([]Decl, len(formulas))
	for index, formula := range formulas {
		body := MustOperExpr(Op("IMPLIES"), Typed(BoolT1), loopApplication, temporalName(initials[index]))
		copy := formula.copyWithTag(formula.Tag())
		copy.SetBody(body)
		newFormulas[index] = copy
	}
	declarations := make([]Decl, 0, len(loop.Module.Declarations())+len(allVariables))
	for _, variable := range allVariables {
		declarations = append(declarations, variable)
	}
	for _, declaration := range loop.Module.Declarations() {
		if _, formula := formulaNames[declaration.Name()]; formula {
			continue
		}
		switch declaration.Name() {
		case newInit.Name():
			declarations = append(declarations, newInit)
		case newNext.Name():
			declarations = append(declarations, newNext)
		case newLoopOK.Name():
			declarations = append(declarations, newLoopOK)
		default:
			declarations = append(declarations, declaration)
		}
	}
	declarations = append(declarations, newFormulas...)
	result := NewModule(loop.Module.Name(), declarations)
	result.properties = cloneMap(loop.Module.properties)
	return SortDeclarations(result)
}

func (encoder *TemporalTableauEncoder) extend(declaration *OperDecl, expression Expr) *OperDecl {
	return encoder.tracker.TrackOperatorDeclaration(func(original *OperDecl) *OperDecl {
		copy := original.copyWithTag(original.Tag())
		copy.SetBody(temporalAnd(original.Body(), expression))
		return copy
	})(declaration)
}

// EncodeTemporalProperties runs the Scala TemporalPass contract: it adds loop
// state, encodes configured temporal properties, and registers their rewritten
// declarations as ordinary invariants for bounded checking.
func EncodeTemporalProperties(
	module *Module,
	predicates DerivedPredicates,
	names *UniqueNameGenerator,
	renamer *IncrementalRenamer,
	tracker TransformationTracker,
) (*Module, DerivedPredicates, error) {
	if len(predicates.TemporalProperties) == 0 {
		return module, predicates, nil
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	if renamer == nil {
		renamer = NewIncrementalRenamer(tracker)
	}
	formulas := make([]*OperDecl, len(predicates.TemporalProperties))
	localInliner := NewInliner(tracker, renamer, false, KeepAllInlinedDeclarations)
	for index, name := range predicates.TemporalProperties {
		declaration, found := declarationNamed(module, name)
		formula, ok := declaration.(*OperDecl)
		if !found || !ok || len(formula.FormalParams()) != 0 {
			return nil, predicates, fmt.Errorf("temporal property %s must be a nullary operator", name)
		}
		body, err := localInliner.Transform(formula.Body())
		if err != nil {
			return nil, predicates, fmt.Errorf("inline temporal property %s: %w", name, err)
		}
		formulas[index] = formula.copyWithTag(formula.Tag())
		formulas[index].SetBody(body)
	}
	loopEncoder := NewTemporalLoopEncoder(tracker)
	loop, err := loopEncoder.AddLoopLogic(module, predicates.Init, predicates.Next)
	if err != nil {
		return nil, predicates, err
	}
	tableau, err := NewTemporalTableauEncoder(loop.Module, names, loopEncoder, tracker)
	if err != nil {
		return nil, predicates, err
	}
	encoded, err := tableau.TemporalsToInvariants(loop, formulas...)
	if err != nil {
		return nil, predicates, err
	}
	predicates.Invariants = append(predicates.Invariants, predicates.TemporalProperties...)
	predicates.TemporalProperties = nil
	return encoded, predicates, nil
}
