package apalache

import (
	"fmt"
	"sort"
)

// StratifiedRewriter lowers a supported ground/state expression into a
// PureArena graph and records the ground constraints separately.
type StratifiedRewriter struct {
	integers    *IntValueCache
	ranges      *IntRangeCache
	literals    *UninterpretedLiteralCache
	defaults    *DefaultValueCache
	equality    *LazyEquality
	constraints []Expr
}

func NewStratifiedRewriter() *StratifiedRewriter {
	result := &StratifiedRewriter{integers: NewIntValueCache(), literals: NewUninterpretedLiteralCache()}
	result.ranges = NewIntRangeCache(result.integers)
	result.equality = NewLazyEquality(result.assert, func(tag string) (ArenaCell, bool) {
		return result.literals.Get(UninterpretedLiteral{Type: StrT1, Index: tag})
	})
	result.defaults = NewDefaultValueCache(result)
	return result
}

func (rewriter *StratifiedRewriter) IntegerCache() *IntValueCache { return rewriter.integers }
func (rewriter *StratifiedRewriter) LiteralCache() *UninterpretedLiteralCache {
	return rewriter.literals
}
func (rewriter *StratifiedRewriter) DefaultValueCache() *DefaultValueCache { return rewriter.defaults }
func (rewriter *StratifiedRewriter) LazyEquality() *LazyEquality           { return rewriter.equality }

func (rewriter *StratifiedRewriter) Constraints() []Expr {
	return append([]Expr(nil), rewriter.constraints...)
}

func (rewriter *StratifiedRewriter) ResetConstraints() {
	rewriter.constraints = nil
	rewriter.equality.Dispose()
	rewriter.defaults.Dispose()
	rewriter.ranges.Dispose()
}

func (rewriter *StratifiedRewriter) assert(expression Expr) {
	rewriter.constraints = append(rewriter.constraints, expression)
}

func (rewriter *StratifiedRewriter) Rewrite(expression Expr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	if expression == nil || scope.Arena == nil {
		return scope, ArenaCell{}, fmt.Errorf("stratified rewriting requires an expression and arena")
	}
	switch typed := expression.(type) {
	case *ValueExpr:
		return rewriter.rewriteValue(typed, scope)
	case *NameExpr:
		return rewriter.rewriteName(typed, scope)
	case *LetInExpr:
		return rewriter.rewriteLetIn(typed, scope)
	case *OperExpr:
		switch typed.Operator().Name() {
		case "Apalache!:=":
			return rewriter.rewriteAssignment(typed, scope)
		case "AND", "OR", "IMPLIES", "EQUIV":
			return rewriter.rewriteBoolean(typed, scope)
		case "NOT":
			return rewriter.rewriteNegation(typed, scope)
		case "EQ", "NE":
			return rewriter.rewriteBasicEquality(typed, scope)
		case "PLUS", "MINUS", "MULT", "DIV", "MOD", "UNARY_MINUS":
			return rewriter.rewriteArithmetic(typed, scope)
		case "POW":
			return rewriter.rewritePower(typed, scope)
		case "INT_RANGE":
			return rewriter.rewriteIntegerRange(typed, scope)
		case "LT", "LE", "GT", "GE":
			return rewriter.rewriteComparison(typed, scope)
		case "IF_THEN_ELSE":
			return rewriter.rewriteConditional(typed, scope)
		case "LABEL":
			return rewriter.rewriteLabel(typed, scope)
		case "SET_ENUM":
			return rewriter.rewriteSetConstructor(typed, scope)
		case "SET_UNION2":
			return rewriter.rewriteSetUnion(typed, scope)
		case "SET_UNARY_UNION":
			return rewriter.rewriteSetUnaryUnion(typed, scope)
		case "SET_INTERSECT", "SET_MINUS":
			return rewriter.rewriteSetBinaryFilter(typed, scope)
		case "SET_SUBSET_EQ":
			return rewriter.rewriteSubset(typed, scope)
		case "SET_POWERSET":
			return rewriter.rewritePowerSet(typed, scope)
		case "FiniteSets!Cardinality":
			return rewriter.rewriteCardinality(typed, scope)
		case "FiniteSets!IsFiniteSet":
			return rewriter.rewriteIsFiniteSet(typed, scope)
		case "Sequences!Len":
			return rewriter.rewriteSequenceLength(typed, scope)
		case "Sequences!Head":
			return rewriter.rewriteSequenceHead(typed, scope)
		case "Sequences!Tail":
			return rewriter.rewriteSequenceTail(typed, scope)
		case "Sequences!Append":
			return rewriter.rewriteSequenceAppend(typed, scope)
		case "Sequences!Concat":
			return rewriter.rewriteSequenceConcat(typed, scope)
		case "Sequences!SubSeq":
			return rewriter.rewriteSubSequence(typed, scope)
		case "SequencesExt!ToSet":
			return rewriter.rewriteSequenceToSet(typed, scope)
		case "Apalache!MkSeq":
			return rewriter.rewriteMakeSequence(typed, scope)
		case "Apalache!ApaFoldSeqLeft":
			return rewriter.rewriteFoldSequence(typed, scope)
		case "Apalache!ApaFoldSet":
			return rewriter.rewriteFoldSet(typed, scope)
		case "Apalache!Repeat":
			return rewriter.rewriteRepeat(typed, scope)
		case "Apalache!Guess", "CHOOSE3":
			return rewriter.rewriteBoundedChoice(typed, scope)
		case "EXISTS3", "FORALL3":
			return rewriter.rewriteFiniteQuantifier(typed, scope)
		case "Apalache!Skolem":
			return rewriter.rewriteSkolemizedExistential(typed, scope)
		case "FUN_CTOR":
			return rewriter.rewriteFunctionConstructor(typed, scope)
		case "DOMAIN":
			return rewriter.rewriteDomain(typed, scope)
		case "SET_FILTER":
			return rewriter.rewriteSetFilter(typed, scope)
		case "SET_MAP":
			return rewriter.rewriteSetMap(typed, scope)
		case "FUN_SET":
			return rewriter.rewriteFunctionSet(typed, scope)
		case "SET_IN", "Apalache!SelectInSet":
			return rewriter.rewriteMembership(typed, scope)
		case "SET_NOT_IN":
			return rewriter.rewriteNegatedMembership(typed, scope)
		case "TUPLE":
			return rewriter.rewriteTuple(typed, scope)
		case "RECORD":
			return rewriter.rewriteRecord(typed, scope)
		case "Variants!Variant":
			return rewriter.rewriteVariant(typed, scope)
		case "Variants!VariantTag":
			return rewriter.rewriteVariantTag(typed, scope)
		case "Variants!VariantGetUnsafe":
			return rewriter.rewriteVariantGetUnsafe(typed, scope)
		case "Variants!VariantGetOrElse":
			return rewriter.rewriteVariantGetOrElse(typed, scope)
		case "Variants!VariantFilter":
			return rewriter.rewriteVariantFilter(typed, scope)
		case "FUN_APP":
			return rewriter.rewriteApplication(typed, scope)
		case "EXCEPT":
			return rewriter.rewriteExcept(typed, scope)
		case "Apalache!SetAsFun":
			return rewriter.rewriteSetAsFunction(typed, scope)
		case "Apalache!Expand":
			return rewriter.rewriteSetExpansion(typed, scope)
		case "Apalache!Gen":
			return rewriter.rewriteGenerate(typed, scope)
		case "Apalache!ConstCardinality":
			return rewriter.rewriteTransparentUnary(typed, scope, "ConstCardinality")
		case "PRIME":
			return rewriter.rewritePrime(typed, scope)
		case "OPER_APP":
			return rewriter.rewriteNullaryOperatorApplication(typed, scope)
		}
		return scope, ArenaCell{}, fmt.Errorf("no stratified rule is applicable to operator %s", typed.Operator().Name())
	}
	return scope, ArenaCell{}, fmt.Errorf("no stratified rule is applicable to %T", expression)
}

func (rewriter *StratifiedRewriter) rewriteNullaryOperatorApplication(
	expression *OperExpr,
	scope RewriterScope,
) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("residual operator application has %d arguments", len(arguments)-1)
	}
	name, ok := arguments[0].(*NameExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("residual nullary operator application has callee %T", arguments[0])
	}
	cell, found := scope.Binding.Get(name.Name())
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("nullary operator %s is unbound", name.Name())
	}
	return scope, cell, nil
}

func (rewriter *StratifiedRewriter) rewriteLetIn(expression *LetInExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	baseBinding := scope.Binding
	localNames := make([]string, 0, len(expression.Declarations()))
	for _, declaration := range expression.Declarations() {
		if len(declaration.FormalParams()) != 0 {
			return scope, ArenaCell{}, fmt.Errorf("cannot directly rewrite parameterized LET operator %s", declaration.Name())
		}
		var value ArenaCell
		var err error
		scope, value, err = rewriter.Rewrite(declaration.Body(), scope)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("rewrite LET operator %s: %w", declaration.Name(), err)
		}
		scope.Binding = scope.Binding.With(declaration.Name(), value)
		localNames = append(localNames, declaration.Name())
	}
	resultScope, result, err := rewriter.Rewrite(expression.Body(), scope)
	binding := resultScope.Binding.Map()
	for _, name := range localNames {
		if previous, shadowed := baseBinding.Get(name); shadowed {
			binding[name] = previous
		} else {
			delete(binding, name)
		}
	}
	resultScope.Binding = NewBinding(binding)
	return resultScope, result, err
}

func (rewriter *StratifiedRewriter) rewriteTransparentUnary(
	expression *OperExpr, scope RewriterScope, name string,
) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("%s requires one operand", name)
	}
	return rewriter.Rewrite(arguments[0], scope)
}

func (rewriter *StratifiedRewriter) rewritePrime(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("prime requires one operand")
	}
	name, ok := arguments[0].(*NameExpr)
	if !ok || IsArenaCellName(name.Name()) {
		return scope, ArenaCell{}, fmt.Errorf("prime is only implemented for variables")
	}
	cell, found := scope.Binding.Get(name.Name() + "'")
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("primed variable %s' is not assigned a value", name.Name())
	}
	return scope, cell, nil
}

func (rewriter *StratifiedRewriter) rewriteLabel(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) < 1 {
		return scope, ArenaCell{}, fmt.Errorf("label requires a labelled expression")
	}
	return rewriter.Rewrite(arguments[0], scope)
}

func (rewriter *StratifiedRewriter) rewriteIsFiniteSet(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	if len(expression.Arguments()) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("IsFiniteSet requires one operand")
	}
	// This deliberately matches Scala: every set admitted into the symbolic
	// finite-set fragment is reported finite, without rewriting its operand.
	return scope, scope.Arena.TrueCell(), nil
}

func (rewriter *StratifiedRewriter) rewriteFunctionConstructor(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("function constructor currently requires one binder and domain")
	}
	binder, ok := arguments[1].(*NameExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("function constructor binder must be a name")
	}
	typ, ok := expression.Tag().Type()
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("function constructor must be typed")
	}
	functionType, ok := typ.(FunType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("function constructor has non-function type %s", typ)
	}
	scope, domain, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, domain, err = rewriter.coerceCell(scope, domain, mustSet(functionType.Argument()))
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("function domain cannot be widened to Set(%s): %w", functionType.Argument(), err)
	}
	domainType, ok := domain.Type.TLAType().(SetType)
	if !ok || !EqualTypes(domainType.Element(), functionType.Argument()) {
		return scope, ArenaCell{}, fmt.Errorf("function domain has type %s, expected Set(%s)", domain.Type, functionType.Argument())
	}
	baseBinding := scope.Binding
	domainPointers := scope.Arena.GetHas(domain)
	relationPointers := make([]ElementPointer, len(domainPointers))
	pairType, _ := NewTupleType(functionType.Argument(), functionType.Result())
	for index, pointer := range domainPointers {
		mappedScope := RewriterScope{Arena: scope.Arena, Binding: baseBinding.With(binder.Name(), pointer.Element())}
		mappedScope, result, rewriteErr := rewriter.Rewrite(arguments[0], mappedScope)
		if rewriteErr != nil {
			return scope, ArenaCell{}, rewriteErr
		}
		mappedScope, result, rewriteErr = rewriter.coerceCell(mappedScope, result, functionType.Result())
		if rewriteErr != nil {
			return scope, ArenaCell{}, fmt.Errorf("function result cannot be widened to %s: %w", functionType.Result(), rewriteErr)
		}
		newArena, pair := mappedScope.Arena.AppendCellType(BasicCellType{Type: pairType})
		mappedScope.Arena = newArena.AppendHas(pair,
			FixedElementPointer{Cell: pointer.Element()}, FixedElementPointer{Cell: result})
		scope.Arena = mappedScope.Arena
		relationPointers[index] = PointerWithSameCondition(pointer, pair)
	}
	scope.Binding = baseBinding
	newArena, relation := scope.Arena.AppendCellType(BasicCellType{Type: functionRelationType(functionType)})
	scope.Arena = newArena.AppendHas(relation, relationPointers...)
	rewriter.addMembershipConstraints(scope.Arena, relation)
	newArena, function := scope.Arena.AppendCellType(BasicCellType{Type: functionType})
	scope.Arena = newArena.SetDomain(function, domain).SetCodomain(function, relation)
	return scope, function, nil
}

func (rewriter *StratifiedRewriter) rewriteDomain(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("DOMAIN requires one operand")
	}
	scope, value, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if isFunctionCellType(value.Type) {
		domain, domainErr := scope.Arena.Domain(value)
		return scope, domain, domainErr
	}
	if isTupleCellType(value.Type) {
		tupleType := value.Type.TLAType().(TupleType)
		elements := make([]Expr, len(tupleType.Elements()))
		for index := range elements {
			elements[index] = NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1))
		}
		return rewriter.Rewrite(MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), elements...), scope)
	}
	if isRecordCellType(value.Type) {
		if _, legacy := value.Type.TLAType().(RecordType); legacy && scope.Arena.HasDomain(value) {
			domain, domainErr := scope.Arena.Domain(value)
			return scope, domain, domainErr
		}
		fields, ok := recordTypeFields(value.Type)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("DOMAIN requires a closed record type")
		}
		keys := sortedStringKeys(fields)
		elements := make([]Expr, len(keys))
		for index, key := range keys {
			elements[index] = NewValueExpr(StringValue(key), Typed(StrT1))
		}
		return rewriter.Rewrite(MustOperExpr(Op("SET_ENUM"), Typed(mustSet(StrT1)), elements...), scope)
	}
	if isSequenceCellType(value.Type) {
		length, _, sequenceElements, unpackErr := unpackSequence(scope.Arena, value)
		if unpackErr != nil {
			return scope, ArenaCell{}, unpackErr
		}
		pointers := make([]ElementPointer, len(sequenceElements))
		for index := range sequenceElements {
			var integer ArenaCell
			scope, integer, err = rewriter.Rewrite(NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1)), scope)
			if err != nil {
				return scope, ArenaCell{}, err
			}
			pointers[index] = SMTElementPointer{Cell: integer, Expression: MustOperExpr(Op("LE"), Typed(BoolT1),
				integer.NameExpr(), length.NameExpr())}
		}
		newArena, domain := scope.Arena.AppendCellType(BasicCellType{Type: mustSet(IntT1)})
		scope.Arena = newArena.AppendHas(domain, pointers...)
		rewriter.addMembershipConstraints(scope.Arena, domain)
		return scope, domain, nil
	}
	return scope, ArenaCell{}, fmt.Errorf("DOMAIN is not yet implemented for %s", value.Type)
}

func (rewriter *StratifiedRewriter) rewriteFiniteQuantifier(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("bounded quantifier requires a name, set, and predicate")
	}
	binder, ok := arguments[0].(*NameExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("bounded quantifier binder must be a name")
	}
	scope, setCell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if _, integerRange := setCell.Type.(IntegerRangeCellType); integerRange {
		scope, setCell, err = rewriter.materializeArenaIntegerRange(scope, setCell)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("bounded quantifier over integer range: %w", err)
		}
	}
	if !isFiniteSetCellType(setCell.Type) {
		return scope, ArenaCell{}, fmt.Errorf("bounded quantifier requires a finite set, found %s", setCell.Type)
	}
	baseBinding := scope.Binding
	pointers := scope.Arena.GetHas(setCell)
	terms := make([]Expr, len(pointers))
	for index, pointer := range pointers {
		predicateScope := RewriterScope{Arena: scope.Arena, Binding: baseBinding.With(binder.Name(), pointer.Element())}
		predicateScope, predicate, rewriteErr := rewriter.Rewrite(arguments[2], predicateScope)
		if rewriteErr != nil {
			return scope, ArenaCell{}, rewriteErr
		}
		if !EqualTypes(predicate.Type.TLAType(), BoolT1) {
			return scope, ArenaCell{}, fmt.Errorf("bounded quantifier predicate has type %s", predicate.Type)
		}
		scope.Arena = predicateScope.Arena
		if expression.Operator().Name() == "EXISTS3" {
			terms[index] = booleanAnd(pointer.ToSMT(), predicate.NameExpr())
		} else {
			terms[index] = booleanOr(MustOperExpr(Op("NOT"), Typed(BoolT1), pointer.ToSMT()), predicate.NameExpr())
		}
	}
	scope.Binding = baseBinding
	condition := booleanAnd(terms...)
	if expression.Operator().Name() == "EXISTS3" {
		condition = booleanOr(terms...)
	}
	scope, result := rewriter.bindBoolean(scope, condition)
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSkolemizedExistential(
	expression *OperExpr,
	scope RewriterScope,
) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("Skolem requires one existential expression")
	}
	exists, ok := arguments[0].(*OperExpr)
	if !ok || exists.Operator().Name() != "EXISTS3" || len(exists.Arguments()) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("Skolem requires a bounded existential expression")
	}
	existsArguments := exists.Arguments()
	binder, ok := existsArguments[0].(*NameExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("Skolem existential binder must be a name")
	}
	scope, setCell, err := rewriter.Rewrite(existsArguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	setType, ok := setCell.Type.TLAType().(SetType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("Skolem existential bound has non-set type %s", setCell.Type)
	}
	guess := MustOperExpr(Op("Apalache!Guess"), Typed(setType.Element()), setCell.NameExpr())
	scope, picked, err := rewriter.Rewrite(guess, scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), picked.NameExpr(), setCell.NameExpr())
	scope, memberCell, err := rewriter.Rewrite(membership, scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	previous, shadowed := scope.Binding.Get(binder.Name())
	scope.Binding = scope.Binding.With(binder.Name(), picked)
	scope, predicate, err := rewriter.Rewrite(existsArguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualTypes(predicate.Type.TLAType(), BoolT1) {
		return scope, ArenaCell{}, fmt.Errorf("Skolem existential predicate has non-Boolean type %s", predicate.Type)
	}
	binding := scope.Binding.Map()
	if shadowed {
		binding[binder.Name()] = previous
	} else {
		delete(binding, binder.Name())
	}
	scope.Binding = NewBinding(binding)
	scope, result := rewriter.bindBoolean(scope, booleanAnd(memberCell.NameExpr(), predicate.NameExpr()))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteBoundedChoice(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	var setExpression Expr
	switch expression.Operator().Name() {
	case "Apalache!Guess":
		if len(arguments) != 1 {
			return scope, ArenaCell{}, fmt.Errorf("Guess requires one set")
		}
		setExpression = arguments[0]
	case "CHOOSE3":
		if len(arguments) != 3 {
			return scope, ArenaCell{}, fmt.Errorf("bounded CHOOSE requires a name, set, and predicate")
		}
		if _, ok := arguments[0].(*NameExpr); !ok {
			return scope, ArenaCell{}, fmt.Errorf("bounded CHOOSE binder must be a name")
		}
		setExpression = MustOperExpr(Op("SET_FILTER"), arguments[1].Tag(), arguments[0], arguments[1], arguments[2])
	}
	scope, setCell, err := rewriter.Rewrite(setExpression, scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	switch setCell.Type.(type) {
	case FiniteFunctionSetCellType, IntegerRangeCellType, PowerSetCellType, InfiniteSetCellType:
		if expression.Operator().Name() != "Apalache!Guess" {
			return scope, ArenaCell{}, fmt.Errorf("bounded CHOOSE over compact or infinite sets is not yet implemented")
		}
		return rewriter.pickFromCompactCollection(scope, setCell)
	}
	setType, ok := setCell.Type.TLAType().(SetType)
	if !ok || !isFiniteSetCellType(setCell.Type) {
		return scope, ArenaCell{}, fmt.Errorf("bounded choice requires a finite set, found %s", setCell.Type)
	}
	pointers := scope.Arena.GetHas(setCell)
	if len(pointers) == 0 {
		return rewriter.defaults.GetOrCreate(scope, setType.Element())
	}
	candidates := make([]ArenaCell, len(pointers))
	for index, pointer := range pointers {
		candidates[index] = pointer.Element()
	}
	scope, oracle, err := CreateIntOracle(scope, len(candidates)+1)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, picked, err := NewCherryPicker(rewriter).Pick(scope, oracle, candidates)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	assertions := make([]Expr, oracle.Size())
	allAbsent := make([]Expr, len(pointers))
	for index, pointer := range pointers {
		assertions[index] = pointer.ToSMT()
		allAbsent[index] = MustOperExpr(Op("NOT"), Typed(BoolT1), pointer.ToSMT())
	}
	assertions[len(candidates)] = booleanAnd(allAbsent...)
	membershipConstraint, err := oracle.Cases(scope, assertions, nil)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	rangeConditions := make([]Expr, oracle.Size())
	for index := range rangeConditions {
		rangeConditions[index], err = oracle.Chosen(scope, index)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	rewriter.assert(booleanAnd(booleanOr(rangeConditions...), membershipConstraint))
	return scope, picked, nil
}

func (rewriter *StratifiedRewriter) selectSequenceElement(scope RewriterScope, elements []ElementPointer, index Expr, defaultElement ArenaCell) (RewriterScope, ArenaCell, error) {
	choice := defaultElement
	for indexBase0, pointer := range elements {
		condition := MustOperExpr(Op("EQ"), Typed(BoolT1), index,
			NewValueExpr(Int64Value(int64(indexBase0+1)), Typed(IntT1)))
		var predicate ArenaCell
		scope, predicate = rewriter.bindBoolean(scope, condition)
		var err error
		scope, choice, err = rewriter.chooseConditional(scope, predicate, pointer.Element(), choice)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	return scope, choice, nil
}

func (rewriter *StratifiedRewriter) rewriteSubSequence(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("subsequence requires a sequence, start, and end")
	}
	scope, sequence, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	length, _, oldElements, err := unpackSequence(scope.Arena, sequence)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, start, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, end, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualTypes(start.Type.TLAType(), IntT1) || !EqualTypes(end.Type.TLAType(), IntT1) {
		return scope, ArenaCell{}, fmt.Errorf("subsequence bounds must be integers")
	}
	zero := NewValueExpr(Int64Value(0), Typed(IntT1))
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	adjustedStartExpression := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("GT"), Typed(BoolT1), start.NameExpr(), zero), start.NameExpr(), one)
	scope, adjustedStart := rewriter.bindInteger(scope, adjustedStartExpression)
	adjustedEndExpression := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("LE"), Typed(BoolT1), end.NameExpr(), length.NameExpr()), end.NameExpr(), length.NameExpr())
	scope, adjustedEnd := rewriter.bindInteger(scope, adjustedEndExpression)
	difference := MustOperExpr(Op("MINUS"), Typed(IntT1), adjustedEnd.NameExpr(), adjustedStart.NameExpr())
	newLengthExpression := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("GE"), Typed(BoolT1), adjustedEnd.NameExpr(), adjustedStart.NameExpr()),
		MustOperExpr(Op("PLUS"), Typed(IntT1), one, difference), zero)
	scope, newLength := rewriter.bindInteger(scope, newLengthExpression)
	sequenceType := sequence.Type.TLAType().(SeqType)
	scope, defaultElement, err := rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	elements := make([]ArenaCell, len(oldElements))
	for index := range elements {
		sourceIndex := MustOperExpr(Op("PLUS"), Typed(IntT1), adjustedStart.NameExpr(),
			NewValueExpr(Int64Value(int64(index)), Typed(IntT1)))
		scope, elements[index], err = rewriter.selectSequenceElement(scope, oldElements, sourceIndex, defaultElement)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, elements, newLength)
	scope.Arena = newArena
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSequenceConcat(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("sequence concatenation requires two operands")
	}
	scope, left, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, right, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	resultType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("sequence concatenation must be typed")
	}
	scope, left, err = rewriter.coerceCell(scope, left, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("widen left sequence: %w", err)
	}
	scope, right, err = rewriter.coerceCell(scope, right, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("widen right sequence: %w", err)
	}
	if !isSequenceCellType(left.Type) || !EqualCellTypes(left.Type, right.Type) {
		return scope, ArenaCell{}, fmt.Errorf("concatenation requires matching sequences, found %s and %s", left.Type, right.Type)
	}
	leftLength, _, leftElements, err := unpackSequence(scope.Arena, left)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	rightLength, _, rightElements, err := unpackSequence(scope.Arena, right)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	sequenceType := left.Type.TLAType().(SeqType)
	scope, defaultElement, err := rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newLengthExpression := MustOperExpr(Op("PLUS"), Typed(IntT1), leftLength.NameExpr(), rightLength.NameExpr())
	scope, newLength := rewriter.bindInteger(scope, newLengthExpression)
	elements := make([]ArenaCell, len(leftElements)+len(rightElements))
	for index := range elements {
		position := NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1))
		rightIndex := MustOperExpr(Op("MINUS"), Typed(IntT1), position, leftLength.NameExpr())
		rightChoice := defaultElement
		for rightIndexBase0, pointer := range rightElements {
			condition := MustOperExpr(Op("EQ"), Typed(BoolT1), rightIndex,
				NewValueExpr(Int64Value(int64(rightIndexBase0+1)), Typed(IntT1)))
			var predicate ArenaCell
			scope, predicate = rewriter.bindBoolean(scope, condition)
			scope, rightChoice, err = rewriter.chooseConditional(scope, predicate, pointer.Element(), rightChoice)
			if err != nil {
				return scope, ArenaCell{}, err
			}
		}
		leftChoice := defaultElement
		if index < len(leftElements) {
			leftChoice = leftElements[index].Element()
		}
		inLeft := MustOperExpr(Op("LE"), Typed(BoolT1), position, leftLength.NameExpr())
		var predicate ArenaCell
		scope, predicate = rewriter.bindBoolean(scope, inLeft)
		scope, elements[index], err = rewriter.chooseConditional(scope, predicate, leftChoice, rightChoice)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, elements, newLength)
	scope.Arena = newArena
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) bindBoolean(scope RewriterScope, condition Expr) (RewriterScope, ArenaCell) {
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), condition))
	return scope, result
}

func (rewriter *StratifiedRewriter) bindInteger(scope RewriterScope, expression Expr) (RewriterScope, ArenaCell) {
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), expression))
	return scope, result
}

func (rewriter *StratifiedRewriter) rewriteSequenceTail(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("sequence tail requires one operand")
	}
	scope, sequence, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	length, _, elements, err := unpackSequence(scope.Arena, sequence)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if len(elements) == 0 {
		return scope, sequence, nil
	}
	newLengthExpression := MustOperExpr(Op("MINUS"), Typed(IntT1), length.NameExpr(), NewValueExpr(Int64Value(1), Typed(IntT1)))
	scope, newLength := rewriter.bindInteger(scope, newLengthExpression)
	tail := make([]ArenaCell, len(elements)-1)
	for index := range tail {
		tail[index] = elements[index+1].Element()
	}
	sequenceType := sequence.Type.TLAType().(SeqType)
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, tail, newLength)
	scope.Arena = newArena
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSequenceAppend(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("sequence append requires two operands")
	}
	scope, sequence, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	length, _, oldElements, err := unpackSequence(scope.Arena, sequence)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, appended, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	sequenceType := sequence.Type.TLAType().(SeqType)
	scope, appended, err = rewriter.coerceCell(scope, appended, sequenceType.Element())
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("appended element cannot be widened to %s: %w", sequenceType.Element(), err)
	}
	newLengthExpression := MustOperExpr(Op("PLUS"), Typed(IntT1), length.NameExpr(), NewValueExpr(Int64Value(1), Typed(IntT1)))
	scope, newLength := rewriter.bindInteger(scope, newLengthExpression)
	elements := make([]ArenaCell, len(oldElements)+1)
	for index, pointer := range oldElements {
		inOldLength := MustOperExpr(Op("LE"), Typed(BoolT1),
			NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1)), length.NameExpr())
		var predicate ArenaCell
		scope, predicate = rewriter.bindBoolean(scope, inOldLength)
		scope, elements[index], err = rewriter.chooseConditional(scope, predicate, pointer.Element(), appended)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	elements[len(oldElements)] = appended
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, elements, newLength)
	scope.Arena = newArena
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSequenceHead(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("sequence head requires one operand")
	}
	scope, sequence, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	_, _, elements, err := unpackSequence(scope.Arena, sequence)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if len(elements) != 0 {
		return scope, elements[0].Element(), nil
	}
	sequenceType := sequence.Type.TLAType().(SeqType)
	return rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
}

func (rewriter *StratifiedRewriter) rewriteSequenceLength(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("sequence length requires one operand")
	}
	scope, sequence, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	length, _, _, err := unpackSequence(scope.Arena, sequence)
	return scope, length, err
}

func (rewriter *StratifiedRewriter) rewriteCardinality(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("cardinality requires one operand")
	}
	scope, setCell, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !isFiniteSetCellType(setCell.Type) {
		return scope, ArenaCell{}, fmt.Errorf("cardinality requires a finite set, found %s", setCell.Type)
	}
	pointers := scope.Arena.GetHas(setCell)
	sum := Expr(NewValueExpr(Int64Value(0), Typed(IntT1)))
	for index, pointer := range pointers {
		uniqueConditions := []Expr{pointer.ToSMT()}
		for previous := 0; previous < index; previous++ {
			equal, equalityErr := rewriter.equality.Equal(scope.Arena, pointer.Element(), pointers[previous].Element())
			if equalityErr != nil {
				return scope, ArenaCell{}, equalityErr
			}
			uniqueConditions = append(uniqueConditions, booleanOr(
				MustOperExpr(Op("NOT"), Typed(BoolT1), pointers[previous].ToSMT()),
				MustOperExpr(Op("NOT"), Typed(BoolT1), equal),
			))
		}
		increment := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), booleanAnd(uniqueConditions...),
			NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(0), Typed(IntT1)))
		sum = MustOperExpr(Op("PLUS"), Typed(IntT1), sum, increment)
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), sum))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSubset(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("subset comparison requires two operands")
	}
	scope, left, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, right, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	commonType, compatible, err := CommonSupertype(left.Type.TLAType(), right.Type.TLAType())
	if err != nil || !compatible {
		return scope, ArenaCell{}, fmt.Errorf("subset comparison requires compatible sets, found %s and %s", left.Type, right.Type)
	}
	scope, left, err = rewriter.coerceCell(scope, left, commonType)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, right, err = rewriter.coerceCell(scope, right, commonType)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !isFiniteSetCellType(left.Type) || !EqualCellTypes(left.Type, right.Type) {
		return scope, ArenaCell{}, fmt.Errorf("subset comparison requires matching finite sets, found %s and %s", left.Type, right.Type)
	}
	condition, err := rewriter.equality.subsetCondition(scope.Arena, left, right)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), condition))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSetBinaryFilter(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("set operator %s requires two operands", expression.Operator().Name())
	}
	scope, left, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, right, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	resultType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("set operator %s must be typed", expression.Operator().Name())
	}
	scope, left, err = rewriter.coerceCell(scope, left, resultType)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, right, err = rewriter.coerceCell(scope, right, resultType)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !isFiniteSetCellType(left.Type) || !EqualCellTypes(left.Type, right.Type) {
		return scope, ArenaCell{}, fmt.Errorf("set operator %s requires matching finite sets, found %s and %s", expression.Operator().Name(), left.Type, right.Type)
	}
	candidates, err := MergeElementPointersByCell(append(scope.Arena.GetHas(left), scope.Arena.GetHas(right)...))
	if err != nil {
		return scope, ArenaCell{}, err
	}
	resultPointers := make([]ElementPointer, len(candidates))
	for index, candidate := range candidates {
		leftIn, membershipErr := rewriter.finiteMembershipCondition(scope.Arena, candidate.Element(), left)
		if membershipErr != nil {
			return scope, ArenaCell{}, membershipErr
		}
		rightIn, membershipErr := rewriter.finiteMembershipCondition(scope.Arena, candidate.Element(), right)
		if membershipErr != nil {
			return scope, ArenaCell{}, membershipErr
		}
		guard := rightIn
		if expression.Operator().Name() == "SET_MINUS" {
			guard = MustOperExpr(Op("NOT"), Typed(BoolT1), rightIn)
		}
		resultPointers[index] = SMTElementPointer{Cell: candidate.Element(), Expression: booleanAnd(leftIn, guard)}
	}
	newArena, result := scope.Arena.AppendCellType(left.Type)
	scope.Arena = newArena.AppendHas(result, resultPointers...)
	rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteApplication(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("function application requires two arguments")
	}
	expectedType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("function application must be typed")
	}
	project := func(current RewriterScope, cell ArenaCell) (RewriterScope, ArenaCell, error) {
		if _, residualVariable := expectedType.(VarType); residualVariable {
			return current, cell, nil
		}
		var projectErr error
		current, cell, projectErr = rewriter.coerceCell(current, cell, expectedType)
		if projectErr != nil {
			return current, ArenaCell{}, fmt.Errorf("application result has type %s, expected %s: %w", cell.Type, expectedType, projectErr)
		}
		return current, cell, nil
	}
	scope, collection, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	fields := scope.Arena.GetHas(collection)
	if isTupleCellType(collection.Type) {
		indexExpression, ok := arguments[1].(*ValueExpr)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("tuple application requires a constant integer index")
		}
		indexValue, ok := indexExpression.Value().(IntValue)
		if !ok || !indexValue.BigInt().IsInt64() {
			return scope, ArenaCell{}, fmt.Errorf("tuple application requires a machine-sized integer index")
		}
		index := indexValue.BigInt().Int64()
		if index < 1 || index > int64(len(fields)) {
			return scope, ArenaCell{}, fmt.Errorf("tuple index %d is outside 1..%d", index, len(fields))
		}
		return project(scope, fields[index-1].Element())
	}
	if isRecordCellType(collection.Type) {
		keyExpression, ok := arguments[1].(*ValueExpr)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("record application requires a constant string key")
		}
		keyValue, ok := keyExpression.Value().(StringValue)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("record application requires a constant string key")
		}
		fieldTypes, ok := recordTypeFields(collection.Type)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("record application requires a closed record type")
		}
		keys := sortedStringKeys(fieldTypes)
		key := string(keyValue)
		index := sort.SearchStrings(keys, key)
		if index == len(keys) || keys[index] != key || index >= len(fields) {
			return scope, ArenaCell{}, fmt.Errorf("record key %q is absent", key)
		}
		return project(scope, fields[index].Element())
	}
	if isSequenceCellType(collection.Type) {
		_, _, elements, unpackErr := unpackSequence(scope.Arena, collection)
		if unpackErr != nil {
			return scope, ArenaCell{}, unpackErr
		}
		if indexExpression, literal := arguments[1].(*ValueExpr); literal {
			indexValue, ok := indexExpression.Value().(IntValue)
			if !ok || !indexValue.BigInt().IsInt64() {
				return scope, ArenaCell{}, fmt.Errorf("sequence application requires an integer index")
			}
			index := indexValue.BigInt().Int64()
			if index >= 1 && index <= int64(len(elements)) {
				return project(scope, elements[index-1].Element())
			}
			sequenceType := collection.Type.TLAType().(SeqType)
			defaultScope, defaultCell, defaultErr := rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
			if defaultErr != nil {
				return scope, ArenaCell{}, defaultErr
			}
			scope = defaultScope
			return project(scope, defaultCell)
		}
		scope, indexCell, err := rewriter.Rewrite(arguments[1], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		if !EqualTypes(indexCell.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("sequence application requires an integer index")
		}
		if len(elements) == 0 {
			sequenceType := collection.Type.TLAType().(SeqType)
			return rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
		}
		candidates := make([]ArenaCell, len(elements))
		for index, pointer := range elements {
			candidates[index] = pointer.Element()
		}
		scope, oracle, err := CreateIntOracle(scope, len(candidates))
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, picked, err := NewCherryPicker(rewriter).Pick(scope, oracle, candidates)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		indexMinusOne := MustOperExpr(Op("MINUS"), Typed(IntT1), indexCell.NameExpr(), NewValueExpr(Int64Value(1), Typed(IntT1)))
		oracleMatches := MustOperExpr(Op("EQ"), Typed(BoolT1), indexMinusOne, oracle.Cell.NameExpr())
		inRange := booleanAnd(
			MustOperExpr(Op("LT"), Typed(BoolT1), NewValueExpr(Int64Value(0), Typed(IntT1)), indexCell.NameExpr()),
			MustOperExpr(Op("LE"), Typed(BoolT1), indexCell.NameExpr(), NewValueExpr(Int64Value(int64(len(candidates))), Typed(IntT1))),
		)
		rewriter.assert(MustOperExpr(Op("EQUIV"), Typed(BoolT1), oracleMatches, inRange))
		return project(scope, picked)
	}
	if isFunctionCellType(collection.Type) {
		functionType := collection.Type.TLAType().(FunType)
		scope, argument, err := rewriter.Rewrite(arguments[1], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, argument, err = rewriter.coerceCell(scope, argument, functionType.Argument())
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("function argument cannot be widened to %s: %w", functionType.Argument(), err)
		}
		relation, err := scope.Arena.Codomain(collection)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		pointers := scope.Arena.GetHas(relation)
		if len(pointers) == 0 {
			defaultScope, defaultCell, defaultErr := rewriter.defaults.GetOrCreate(scope, functionType.Result())
			if defaultErr != nil {
				return scope, ArenaCell{}, defaultErr
			}
			return project(defaultScope, defaultCell)
		}
		results := make([]ArenaCell, len(pointers))
		matches := make([]Expr, len(pointers))
		for index, pointer := range pointers {
			pairArgument, pairResult, pairErr := unpackFunctionPair(scope.Arena, pointer.Element())
			if pairErr != nil {
				return scope, ArenaCell{}, pairErr
			}
			results[index] = pairResult
			equal, equalityErr := rewriter.equality.Equal(scope.Arena, argument, pairArgument)
			if equalityErr != nil {
				return scope, ArenaCell{}, equalityErr
			}
			matches[index] = booleanAnd(pointer.ToSMT(), equal)
		}
		scope, oracle, err := CreateIntOracle(scope, len(results)+1)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, picked, err := NewCherryPicker(rewriter).Pick(scope, oracle, results)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		assertions := make([]Expr, oracle.Size())
		copy(assertions, matches)
		notMatches := make([]Expr, len(matches))
		for index, match := range matches {
			notMatches[index] = MustOperExpr(Op("NOT"), Typed(BoolT1), match)
		}
		assertions[len(matches)] = booleanAnd(notMatches...)
		matchConstraint, err := oracle.Cases(scope, assertions, nil)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		rangeConditions := make([]Expr, oracle.Size())
		for index := range rangeConditions {
			rangeConditions[index], err = oracle.Chosen(scope, index)
			if err != nil {
				return scope, ArenaCell{}, err
			}
		}
		rewriter.assert(booleanAnd(booleanOr(rangeConditions...), matchConstraint))
		return project(scope, picked)
	}
	return scope, ArenaCell{}, fmt.Errorf("application is not yet implemented for %s", collection.Type)
}

func (rewriter *StratifiedRewriter) rewriteRecord(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	cellType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	fieldTypes, ok := recordTypeFields(cellType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("record constructor requires a closed record type, found %s", cellType)
	}
	arguments := expression.Arguments()
	if len(arguments)%2 != 0 {
		return scope, ArenaCell{}, fmt.Errorf("record constructor requires key/value pairs")
	}
	values := make(map[string]ArenaCell, len(arguments)/2)
	for index := 0; index < len(arguments); index += 2 {
		keyExpression, ok := arguments[index].(*ValueExpr)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("record key %d is not a string literal", index/2+1)
		}
		keyValue, ok := keyExpression.Value().(StringValue)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("record key %d is not a string literal", index/2+1)
		}
		key := string(keyValue)
		expectedType, found := fieldTypes[key]
		if !found {
			return scope, ArenaCell{}, fmt.Errorf("record key %q is absent from type %s", key, cellType)
		}
		if _, duplicate := values[key]; duplicate {
			return scope, ArenaCell{}, fmt.Errorf("duplicate record key %q", key)
		}
		var value ArenaCell
		scope, value, err = rewriter.Rewrite(arguments[index+1], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, value, err = rewriter.coerceCell(scope, value, expectedType)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("record field %q has type %s, expected %s: %w", key, value.Type, expectedType, err)
		}
		values[key] = value
	}
	recordType, legacy := cellType.TLAType().(RecordType)
	if !legacy && len(values) != len(fieldTypes) {
		return scope, ArenaCell{}, fmt.Errorf("record constructor provides %d of %d fields", len(values), len(fieldTypes))
	}
	keys := sortedStringKeys(fieldTypes)
	pointers := make([]ElementPointer, len(keys))
	for index, key := range keys {
		value, found := values[key]
		if !found {
			scope, value, err = rewriter.defaults.GetOrCreate(scope, recordType.Fields()[key])
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("default record field %q: %w", key, err)
			}
		}
		pointers[index] = FixedElementPointer{Cell: value}
	}
	newArena, record := scope.Arena.AppendCellType(cellType)
	scope.Arena = newArena.AppendHas(record, pointers...)
	if legacy {
		domainElements := make([]Expr, 0, len(values))
		valueKeys := make([]string, 0, len(values))
		for key := range values {
			valueKeys = append(valueKeys, key)
		}
		sort.Strings(valueKeys)
		for _, key := range valueKeys {
			domainElements = append(domainElements, NewValueExpr(StringValue(key), Typed(StrT1)))
		}
		domainExpression := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(StrT1)), domainElements...)
		var domain ArenaCell
		scope, domain, err = rewriter.Rewrite(domainExpression, scope)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("construct record domain: %w", err)
		}
		scope.Arena = scope.Arena.SetDomain(record, domain)
	}
	return scope, record, nil
}

func (rewriter *StratifiedRewriter) rewriteTuple(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	typ, ok := expression.Tag().Type()
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("tuple constructor must be typed")
	}
	if sequenceType, sequence := typ.(SeqType); sequence {
		arguments := expression.Arguments()
		elements := make([]ArenaCell, len(arguments))
		for index, argument := range arguments {
			var err error
			scope, elements[index], err = rewriter.Rewrite(argument, scope)
			if err != nil {
				return scope, ArenaCell{}, err
			}
			scope, elements[index], err = rewriter.coerceCell(scope, elements[index], sequenceType.Element())
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("sequence element %d cannot be widened to %s: %w", index+1, sequenceType.Element(), err)
			}
		}
		arena, sequenceCell, err := makeSequence(scope.Arena, sequenceType, elements, rewriter.integers)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope.Arena = arena
		length, _, _, err := unpackSequence(scope.Arena, sequenceCell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1),
			length.NameExpr(), NewValueExpr(Int64Value(int64(len(elements))), Typed(IntT1))))
		return scope, sequenceCell, nil
	}
	tupleType, ok := typ.(TupleType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("TUPLE has unsupported result type %s", typ)
	}
	types := tupleType.Elements()
	arguments := expression.Arguments()
	if len(arguments) != len(types) {
		return scope, ArenaCell{}, fmt.Errorf("tuple type has arity %d but constructor has %d elements", len(types), len(arguments))
	}
	elements := make([]ArenaCell, len(arguments))
	for index, argument := range arguments {
		var err error
		scope, elements[index], err = rewriter.Rewrite(argument, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, elements[index], err = rewriter.coerceCell(scope, elements[index], types[index])
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("tuple element %d cannot be widened to %s: %w", index+1, types[index], err)
		}
	}
	newArena, tuple := scope.Arena.AppendCellType(BasicCellType{Type: tupleType})
	pointers := make([]ElementPointer, len(elements))
	for index, element := range elements {
		pointers[index] = FixedElementPointer{Cell: element}
	}
	scope.Arena = newArena.AppendHas(tuple, pointers...)
	return scope, tuple, nil
}

func (rewriter *StratifiedRewriter) rewriteNegation(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("negation requires one argument")
	}
	scope, argument, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if argument.Equal(scope.Arena.TrueCell()) {
		return scope, scope.Arena.FalseCell(), nil
	}
	if argument.Equal(scope.Arena.FalseCell()) {
		return scope, scope.Arena.TrueCell(), nil
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), MustOperExpr(Op("NOT"), Typed(BoolT1), argument.NameExpr())))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteNegatedMembership(
	expression *OperExpr,
	scope RewriterScope,
) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("set non-membership requires two operands")
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), arguments...)
	scope, member, err := rewriter.Rewrite(membership, scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if member.Equal(scope.Arena.TrueCell()) {
		return scope, scope.Arena.FalseCell(), nil
	}
	if member.Equal(scope.Arena.FalseCell()) {
		return scope, scope.Arena.TrueCell(), nil
	}
	scope, result := rewriter.bindBoolean(scope, MustOperExpr(Op("NOT"), Typed(BoolT1), member.NameExpr()))
	return scope, result, nil
}

func isDirectSMTCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	switch basic.Type.(type) {
	case ConstType:
		return true
	case primitiveType:
		return EqualTypes(basic.Type, BoolT1) || EqualTypes(basic.Type, IntT1) || EqualTypes(basic.Type, StrT1)
	default:
		return false
	}
}

func (rewriter *StratifiedRewriter) rewriteBasicEquality(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("equality requires two arguments")
	}
	if EqualExpr(arguments[0], arguments[1]) {
		if expression.Operator().Name() == "NE" {
			return scope, scope.Arena.FalseCell(), nil
		}
		return scope, scope.Arena.TrueCell(), nil
	}
	scope, left, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, right, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualCellTypes(left.Type, right.Type) {
		commonType, compatible, unifyErr := CommonSupertype(left.Type.TLAType(), right.Type.TLAType())
		if unifyErr != nil {
			return scope, ArenaCell{}, fmt.Errorf("find common equality type: %w", unifyErr)
		}
		if !compatible {
			return scope, ArenaCell{}, fmt.Errorf("cannot compare cells of types %s and %s", left.Type, right.Type)
		}
		scope, left, err = rewriter.coerceCell(scope, left, commonType)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("coerce left equality operand: %w", err)
		}
		scope, right, err = rewriter.coerceCell(scope, right, commonType)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("coerce right equality operand: %w", err)
		}
	}
	comparison, err := rewriter.equality.Equal(scope.Arena, left, right)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if expression.Operator().Name() == "NE" {
		comparison = MustOperExpr(Op("NOT"), Typed(BoolT1), comparison)
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), comparison))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteArithmetic(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	terms := make([]Expr, len(arguments))
	for index, argument := range arguments {
		var cell ArenaCell
		var err error
		scope, cell, err = rewriter.Rewrite(argument, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		if !EqualTypes(cell.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("integer operator %s received %s", expression.Operator().Name(), cell.Type)
		}
		terms[index] = cell.NameExpr()
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
	scope.Arena = newArena
	operation := MustOperExpr(expression.Operator(), Typed(IntT1), terms...)
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), operation))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewritePower(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("integer exponentiation requires two arguments")
	}
	exponent, constant := constantInteger(arguments[1])
	if !constant {
		scope, base, err := rewriter.Rewrite(arguments[0], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, symbolicExponent, err := rewriter.Rewrite(arguments[1], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		if !EqualTypes(base.Type.TLAType(), IntT1) || !EqualTypes(symbolicExponent.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("integer exponentiation received operand types %s and %s", base.Type, symbolicExponent.Type)
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
		scope.Arena = newArena
		power := MustOperExpr(Op("POW"), Typed(IntT1), base.NameExpr(), symbolicExponent.NameExpr())
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), power))
		return scope, result, nil
	}
	if exponent.Sign() < 0 {
		return scope, ArenaCell{}, fmt.Errorf("integer exponentiation requires a nonnegative exponent, found %s", exponent)
	}
	if !exponent.IsUint64() || exponent.Uint64() > 65535 {
		return scope, ArenaCell{}, fmt.Errorf("integer exponent %s exceeds the supported bound 65535", exponent)
	}
	scope, base, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualTypes(base.Type.TLAType(), IntT1) {
		return scope, ArenaCell{}, fmt.Errorf("integer exponentiation received base type %s", base.Type)
	}
	scope, result, err := rewriter.Rewrite(NewValueExpr(Int64Value(1), Typed(IntT1)), scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	factor := base
	power := exponent.Uint64()
	for power != 0 {
		if power&1 != 0 {
			scope, result = rewriter.multiplyCells(scope, result, factor)
		}
		power >>= 1
		if power != 0 {
			scope, factor = rewriter.multiplyCells(scope, factor, factor)
		}
	}
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) multiplyCells(scope RewriterScope, left, right ArenaCell) (RewriterScope, ArenaCell) {
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
	scope.Arena = newArena
	product := MustOperExpr(Op("MULT"), Typed(IntT1), left.NameExpr(), right.NameExpr())
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), product))
	return scope, result
}

func (rewriter *StratifiedRewriter) rewriteComparison(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("integer comparison requires two arguments")
	}
	terms := make([]Expr, 2)
	for index, argument := range arguments {
		var cell ArenaCell
		var err error
		scope, cell, err = rewriter.Rewrite(argument, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		if !EqualTypes(cell.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("integer comparison received %s", cell.Type)
		}
		terms[index] = cell.NameExpr()
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	comparison := MustOperExpr(expression.Operator(), Typed(BoolT1), terms...)
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), comparison))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteConditional(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("conditional requires three arguments")
	}
	scope, predicate, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if predicate.Equal(scope.Arena.TrueCell()) {
		return rewriter.Rewrite(arguments[1], scope)
	}
	if predicate.Equal(scope.Arena.FalseCell()) {
		return rewriter.Rewrite(arguments[2], scope)
	}
	scope, thenCell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, elseCell, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !EqualTypes(predicate.Type.TLAType(), BoolT1) {
		return scope, ArenaCell{}, fmt.Errorf("conditional predicate has non-Boolean type %s", predicate.Type)
	}
	expectedType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("conditional must be typed")
	}
	scope, thenCell, err = rewriter.coerceCell(scope, thenCell, expectedType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("widen conditional then branch: %w", err)
	}
	scope, elseCell, err = rewriter.coerceCell(scope, elseCell, expectedType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("widen conditional else branch: %w", err)
	}
	if !EqualCellTypes(thenCell.Type, elseCell.Type) {
		return scope, ArenaCell{}, fmt.Errorf("conditional branches have different types %s and %s", thenCell.Type, elseCell.Type)
	}
	return rewriter.chooseConditional(scope, predicate, thenCell, elseCell)
}

func (rewriter *StratifiedRewriter) chooseConditional(scope RewriterScope, predicate, thenCell, elseCell ArenaCell) (RewriterScope, ArenaCell, error) {
	if !EqualCellTypes(thenCell.Type, elseCell.Type) {
		return scope, ArenaCell{}, fmt.Errorf("conditional choices have different types %s and %s", thenCell.Type, elseCell.Type)
	}
	if thenCell.Equal(elseCell) {
		return scope, thenCell, nil
	}
	if isDirectSMTCellType(thenCell.Type) {
		newArena, result := scope.Arena.AppendCellType(thenCell.Type)
		scope.Arena = newArena
		choice := MustOperExpr(Op("IF_THEN_ELSE"), thenCell.NameExpr().Tag(), predicate.NameExpr(), thenCell.NameExpr(), elseCell.NameExpr())
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), choice))
		return scope, result, nil
	}
	if isFiniteSetCellType(thenCell.Type) {
		thenPointers := scope.Arena.GetHas(thenCell)
		elsePointers := scope.Arena.GetHas(elseCell)
		pointers := make([]ElementPointer, 0, len(thenPointers)+len(elsePointers))
		for _, pointer := range thenPointers {
			pointers = append(pointers, pointer.Restrict(predicate.NameExpr()))
		}
		notPredicate := MustOperExpr(Op("NOT"), Typed(BoolT1), predicate.NameExpr())
		for _, pointer := range elsePointers {
			pointers = append(pointers, pointer.Restrict(notPredicate))
		}
		merged, err := MergeElementPointersByCell(pointers)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		newArena, result := scope.Arena.AppendCellType(thenCell.Type)
		scope.Arena = newArena.AppendHas(result, merged...)
		rewriter.addMembershipConstraints(scope.Arena, result)
		return scope, result, nil
	}
	if isTupleCellType(thenCell.Type) || isRecordCellType(thenCell.Type) || isVariantCellType(thenCell.Type) {
		thenFields := scope.Arena.GetHas(thenCell)
		elseFields := scope.Arena.GetHas(elseCell)
		if len(thenFields) != len(elseFields) {
			return scope, ArenaCell{}, fmt.Errorf("conditional structural branches have different arities %d and %d", len(thenFields), len(elseFields))
		}
		fields := make([]ArenaCell, len(thenFields))
		for index := range thenFields {
			var err error
			scope, fields[index], err = rewriter.chooseConditional(scope, predicate, thenFields[index].Element(), elseFields[index].Element())
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("choose conditional structural field %d: %w", index+1, err)
			}
		}
		newArena, result := scope.Arena.AppendCellType(thenCell.Type)
		pointers := make([]ElementPointer, len(fields))
		for index, field := range fields {
			pointers[index] = FixedElementPointer{Cell: field}
		}
		scope.Arena = newArena.AppendHas(result, pointers...)
		if _, legacy := thenCell.Type.TLAType().(RecordType); legacy {
			thenDomain, domainErr := scope.Arena.Domain(thenCell)
			if domainErr != nil {
				return scope, ArenaCell{}, domainErr
			}
			elseDomain, domainErr := scope.Arena.Domain(elseCell)
			if domainErr != nil {
				return scope, ArenaCell{}, domainErr
			}
			var domain ArenaCell
			scope, domain, domainErr = rewriter.chooseConditional(scope, predicate, thenDomain, elseDomain)
			if domainErr != nil {
				return scope, ArenaCell{}, fmt.Errorf("choose conditional record domain: %w", domainErr)
			}
			scope.Arena = scope.Arena.SetDomain(result, domain)
		}
		return scope, result, nil
	}
	if isSequenceCellType(thenCell.Type) {
		thenLength, _, thenElements, err := unpackSequence(scope.Arena, thenCell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		elseLength, _, elseElements, err := unpackSequence(scope.Arena, elseCell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, length, err := rewriter.chooseConditional(scope, predicate, thenLength, elseLength)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		sequenceType := thenCell.Type.TLAType().(SeqType)
		scope, defaultElement, err := rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
		if err != nil {
			return scope, ArenaCell{}, err
		}
		capacity := len(thenElements)
		if len(elseElements) > capacity {
			capacity = len(elseElements)
		}
		elements := make([]ArenaCell, capacity)
		for index := 0; index < capacity; index++ {
			thenElement, elseElement := defaultElement, defaultElement
			if index < len(thenElements) {
				thenElement = thenElements[index].Element()
			}
			if index < len(elseElements) {
				elseElement = elseElements[index].Element()
			}
			scope, elements[index], err = rewriter.chooseConditional(scope, predicate, thenElement, elseElement)
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("choose conditional sequence element %d: %w", index+1, err)
			}
		}
		newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, elements, length)
		scope.Arena = newArena
		return scope, result, nil
	}
	if isFunctionCellType(thenCell.Type) {
		scope, oracle, err := CreateIntOracle(scope, 2)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		elseChosen, _ := oracle.Chosen(scope, 0)
		thenChosen, _ := oracle.Chosen(scope, 1)
		rewriter.assert(booleanAnd(
			booleanOr(elseChosen, thenChosen),
			MustOperExpr(Op("EQUIV"), Typed(BoolT1), predicate.NameExpr(), thenChosen),
		))
		return NewCherryPicker(rewriter).Pick(scope, oracle, []ArenaCell{elseCell, thenCell})
	}
	return scope, ArenaCell{}, fmt.Errorf("conditional branches have unsupported structural type %s", thenCell.Type)
}

func (rewriter *StratifiedRewriter) rewriteValue(expression *ValueExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	switch value := expression.Value().(type) {
	case BoolValue:
		if bool(value) {
			return scope, scope.Arena.TrueCell(), nil
		}
		return scope, scope.Arena.FalseCell(), nil
	case PredefinedSet:
		switch value {
		case BoolSet:
			return scope, scope.Arena.BooleanSetCell(), nil
		case NatSet:
			return scope, scope.Arena.NatSetCell(), nil
		case IntSet:
			return scope, scope.Arena.IntSetCell(), nil
		}
	case IntValue:
		arena, cell, err := rewriter.integers.GetOrCreate(scope.Arena, value.BigInt())
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope.Arena = arena
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), cell.NameExpr(), NewValueExpr(value, Typed(IntT1))))
		return scope, cell, nil
	case StringValue:
		literalType := Type(StrT1)
		if tagged, ok := expression.Tag().Type(); ok {
			if _, constant := tagged.(ConstType); constant {
				literalType = tagged
			}
		}
		literal := UninterpretedLiteral{Type: literalType, Index: string(value)}
		arena, cell, err := rewriter.literals.GetOrCreate(scope.Arena, literal)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope.Arena = arena
		distinct, err := rewriter.literals.ConstraintFor(literal, cell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		rewriter.assert(distinct)
		return scope, cell, nil
	}
	return scope, ArenaCell{}, fmt.Errorf("unsupported stratified literal %T", expression.Value())
}

func (rewriter *StratifiedRewriter) rewriteName(expression *NameExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	if IsArenaCellName(expression.Name()) {
		cell, err := scope.Arena.FindCellByName(expression.Name())
		return scope, cell, err
	}
	cell, found := scope.Binding.Get(expression.Name())
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("variable %s is not assigned a value", expression.Name())
	}
	return scope, cell, nil
}

func (rewriter *StratifiedRewriter) rewriteAssignment(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("assignment requires two arguments")
	}
	prime, ok := arguments[0].(*OperExpr)
	if !ok || prime.Operator().Name() != "PRIME" || len(prime.Arguments()) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("assignment left-hand side must be a primed name")
	}
	name, ok := prime.Arguments()[0].(*NameExpr)
	if !ok || IsArenaCellName(name.Name()) {
		return scope, ArenaCell{}, fmt.Errorf("assignment left-hand side must be a variable name")
	}
	primedName := name.Name() + "'"
	if scope.Binding.Contains(primedName) {
		return scope, ArenaCell{}, fmt.Errorf("variable %s is already assigned", primedName)
	}
	newScope, cell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newScope.Binding = newScope.Binding.With(primedName, cell)
	return newScope, newScope.Arena.TrueCell(), nil
}

func (rewriter *StratifiedRewriter) rewriteBoolean(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) == 0 {
		if expression.Operator().Name() == "AND" {
			return scope, scope.Arena.TrueCell(), nil
		}
		return scope, scope.Arena.FalseCell(), nil
	}
	if len(arguments) == 1 {
		return rewriter.Rewrite(arguments[0], scope)
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	terms := make([]Expr, len(arguments))
	for index, argument := range arguments {
		var cell ArenaCell
		var err error
		scope, cell, err = rewriter.Rewrite(argument, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		terms[index] = cell.NameExpr()
	}
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), MustOperExpr(expression.Operator(), Typed(BoolT1), terms...)))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSetConstructor(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	cellType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	setType, ok := cellType.TLAType().(SetType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("set constructor has non-set type %s", cellType.TLAType())
	}
	unique := make(map[int]ArenaCell)
	for _, element := range expression.Arguments() {
		var cell ArenaCell
		scope, cell, err = rewriter.Rewrite(element, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, cell, err = rewriter.coerceCell(scope, cell, setType.Element())
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("set element has type %s, expected %s: %w", cell.Type, setType.Element(), err)
		}
		unique[cell.ID] = cell
	}
	ids := make([]int, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	newArena, setCell := scope.Arena.AppendCellType(cellType)
	pointers := make([]ElementPointer, len(ids))
	for index, id := range ids {
		pointers[index] = FixedElementPointer{Cell: unique[id]}
	}
	scope.Arena = newArena.AppendHas(setCell, pointers...)
	rewriter.addMembershipConstraints(scope.Arena, setCell)
	return scope, setCell, nil
}

func (rewriter *StratifiedRewriter) rewriteSetUnion(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("set union requires two operands")
	}
	leftScope, left, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	rightScope, right, err := rewriter.Rewrite(arguments[1], leftScope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	resultType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("set union must be typed")
	}
	rightScope, left, err = rewriter.coerceCell(rightScope, left, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("widen left set union operand: %w", err)
	}
	rightScope, right, err = rewriter.coerceCell(rightScope, right, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("widen right set union operand: %w", err)
	}
	pointers, err := MergeElementPointersByCell(append(rightScope.Arena.GetHas(left), rightScope.Arena.GetHas(right)...))
	if err != nil {
		return scope, ArenaCell{}, err
	}
	cellType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, setCell := rightScope.Arena.AppendCellType(cellType)
	rightScope.Arena = newArena.AppendHas(setCell, pointers...)
	rewriter.addMembershipConstraints(rightScope.Arena, setCell)
	return rightScope, setCell, nil
}

func (rewriter *StratifiedRewriter) rewriteSetFilter(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("set filter requires a bound name, set, and predicate")
	}
	variable, ok := arguments[0].(*NameExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("set filter binder must be a name")
	}
	if value, ok := arguments[1].(*ValueExpr); ok {
		if predefined, ok := value.Value().(PredefinedSet); ok && (predefined == IntSet || predefined == NatSet) {
			return scope, ArenaCell{}, fmt.Errorf("set filter over infinite set %s is not supported", predefined.Name())
		}
	}
	setScope, setCell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	baseBinding := setScope.Binding
	pointers := setScope.Arena.GetHas(setCell)
	restricted := make([]ElementPointer, len(pointers))
	for index, pointer := range pointers {
		predicateScope := RewriterScope{Arena: setScope.Arena, Binding: baseBinding.With(variable.Name(), pointer.Element())}
		predicateScope, predicateCell, rewriteErr := rewriter.Rewrite(arguments[2], predicateScope)
		if rewriteErr != nil {
			return scope, ArenaCell{}, rewriteErr
		}
		setScope.Arena = predicateScope.Arena
		restricted[index] = pointer.Restrict(predicateCell.NameExpr())
	}
	setScope.Binding = baseBinding
	cellType, err := CellTypeFromTag(arguments[1].Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, result := setScope.Arena.AppendCellType(cellType)
	setScope.Arena = newArena.AppendHas(result, restricted...)
	rewriter.addMembershipConstraints(setScope.Arena, result)
	return setScope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteMembership(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("set membership requires two arguments")
	}
	scope, element, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, setCell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if setCell.Equal(scope.Arena.IntSetCell()) {
		return scope, scope.Arena.TrueCell(), nil
	}
	if setCell.Equal(scope.Arena.NatSetCell()) {
		if !EqualTypes(element.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("Nat membership requires an integer element")
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
		scope.Arena = newArena
		zero := NewValueExpr(Int64Value(0), Typed(IntT1))
		comparison := MustOperExpr(Op("GE"), Typed(BoolT1), element.NameExpr(), zero)
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), comparison))
		return scope, result, nil
	}
	if _, integerRange := setCell.Type.(IntegerRangeCellType); integerRange {
		if !EqualTypes(element.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("integer range membership requires an integer element")
		}
		lower, rangeErr := scope.Arena.Domain(setCell)
		if rangeErr != nil {
			return scope, ArenaCell{}, rangeErr
		}
		upper, rangeErr := scope.Arena.Codomain(setCell)
		if rangeErr != nil {
			return scope, ArenaCell{}, rangeErr
		}
		condition := booleanAnd(
			MustOperExpr(Op("LE"), Typed(BoolT1), lower.NameExpr(), element.NameExpr()),
			MustOperExpr(Op("LE"), Typed(BoolT1), element.NameExpr(), upper.NameExpr()))
		scope, result := rewriter.bindBoolean(scope, condition)
		return scope, result, nil
	}
	if _, powerSet := setCell.Type.(PowerSetCellType); powerSet {
		if !isFiniteSetCellType(element.Type) {
			return scope, ArenaCell{}, fmt.Errorf("powerset membership requires a finite set element, found %s", element.Type)
		}
		domain, domainErr := scope.Arena.Domain(setCell)
		if domainErr != nil {
			return scope, ArenaCell{}, domainErr
		}
		scope, element, err = rewriter.coerceCell(scope, element, domain.Type.TLAType())
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("powerset membership element cannot be widened to %s: %w", domain.Type, err)
		}
		condition, subsetErr := rewriter.equality.subsetCondition(scope.Arena, element, domain)
		if subsetErr != nil {
			return scope, ArenaCell{}, subsetErr
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
		scope.Arena = newArena
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), condition))
		return scope, result, nil
	}
	setType, ok := setCell.Type.(BasicCellType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("membership is not implemented for arena type %s", setCell.Type)
	}
	tlaSetType, ok := setType.Type.(SetType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("membership right-hand side has non-set type %s", setType.Type)
	}
	scope, element, err = rewriter.coerceCell(scope, element, tlaSetType.Element())
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("membership element has type %s, expected %s: %w", element.Type, tlaSetType.Element(), err)
	}
	membership, err := rewriter.finiteMembershipCondition(scope.Arena, element, setCell)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if value, ok := membership.(*ValueExpr); ok && value.Value() == BoolValue(false) {
		return scope, scope.Arena.FalseCell(), nil
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), membership))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) finiteMembershipCondition(arena *PureArena, element, setCell ArenaCell) (Expr, error) {
	pointers := arena.GetHas(setCell)
	disjuncts := make([]Expr, len(pointers))
	for index, pointer := range pointers {
		equal, err := rewriter.equality.Equal(arena, element, pointer.Element())
		if err != nil {
			return nil, fmt.Errorf("compare %s with candidate %s in %s: %w", element, pointer.Element(), setCell, err)
		}
		disjuncts[index] = booleanAnd(pointer.ToSMT(), equal)
	}
	return booleanOr(disjuncts...), nil
}

func (rewriter *StratifiedRewriter) addMembershipConstraints(arena *PureArena, setCell ArenaCell) {
	for _, pointer := range arena.GetHas(setCell) {
		membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), pointer.Element().NameExpr(), setCell.NameExpr())
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), membership, pointer.ToSMT()))
	}
}
