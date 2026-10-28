package apalache

import (
	"fmt"
	"math/big"
	"sync"
)

// LegacyBuilderExpr is the intermediate builder algebra used by the original
// lir.Builder and its TypedPredefs/UntypedPredefs compilation layers.
type LegacyBuilderExpr interface{ legacyBuilderExpr() }

type LegacyAlias struct {
	Target LegacyBuilderExpr
	Name   string
}
type LegacyWrappedExpr struct{ Expr Expr }
type LegacyValueExpr struct{ Value Value }
type LegacyNameExpr struct{ Name string }
type LegacyOperatorExpr struct {
	Operator  Operator
	Arguments []LegacyBuilderExpr
}
type LegacyLetExpr struct {
	Body         LegacyBuilderExpr
	Declarations []*OperDecl
}

func (LegacyAlias) legacyBuilderExpr()        {}
func (LegacyWrappedExpr) legacyBuilderExpr()  {}
func (LegacyValueExpr) legacyBuilderExpr()    {}
func (LegacyNameExpr) legacyBuilderExpr()     {}
func (LegacyOperatorExpr) legacyBuilderExpr() {}
func (LegacyLetExpr) legacyBuilderExpr()      {}

func WithLegacyAlias(target LegacyBuilderExpr, alias string) (LegacyBuilderExpr, error) {
	if existing, ok := target.(LegacyAlias); ok {
		return nil, NewBuilderError(fmt.Sprintf("Internal builder error: Trying to overwrite alias %s with %s", existing.Name, alias))
	}
	if target == nil {
		return nil, NewBuilderError("cannot alias a nil builder expression")
	}
	return LegacyAlias{Target: target, Name: alias}, nil
}

type LegacyBuilderDecl interface{ legacyBuilderDecl() }

type LegacyOperDecl struct {
	Name         string
	FormalParams []OperatorParam
	Body         LegacyBuilderExpr
}

func (LegacyOperDecl) legacyBuilderDecl() {}

// LegacyBuilder constructs the deliberately untyped intermediate algebra.
// Its expressions are compiled explicitly with BuildLegacyUntyped or
// BuildLegacyTyped, replacing Scala's implicit conversions.
type LegacyBuilder struct{}

func NewLegacyBuilder() *LegacyBuilder { return &LegacyBuilder{} }

// LegacyTLA is the deprecated convenience singleton corresponding to
// lir.convenience.tla. New code should use TypedBuilder or ScopedBuilder.
var LegacyTLA = NewLegacyBuilder()

func (*LegacyBuilder) FromExpr(expr Expr) LegacyBuilderExpr { return LegacyWrappedExpr{Expr: expr} }
func WrapLegacyExpressions(expressions []Expr) []LegacyBuilderExpr {
	result := make([]LegacyBuilderExpr, len(expressions))
	for index, expression := range expressions {
		result[index] = LegacyWrappedExpr{Expr: expression}
	}
	return result
}
func (*LegacyBuilder) Name(name string) LegacyBuilderExpr { return LegacyNameExpr{Name: name} }
func (*LegacyBuilder) BigInt(value *big.Int) LegacyBuilderExpr {
	return LegacyValueExpr{Value: NewIntValue(value)}
}
func (*LegacyBuilder) Int64(value int64) LegacyBuilderExpr {
	return LegacyValueExpr{Value: Int64Value(value)}
}
func (*LegacyBuilder) Decimal(text string) (LegacyBuilderExpr, error) {
	value, err := NewDecimalValue(text)
	if err != nil {
		return nil, err
	}
	return LegacyValueExpr{Value: value}, nil
}
func (*LegacyBuilder) Bool(value bool) LegacyBuilderExpr {
	return LegacyValueExpr{Value: BoolValue(value)}
}
func (*LegacyBuilder) String(value string) LegacyBuilderExpr {
	return LegacyValueExpr{Value: StringValue(value)}
}
func (*LegacyBuilder) BooleanSet() LegacyBuilderExpr { return LegacyValueExpr{Value: BoolSet} }
func (*LegacyBuilder) StringSet() LegacyBuilderExpr  { return LegacyValueExpr{Value: StringSet} }
func (*LegacyBuilder) IntSet() LegacyBuilderExpr     { return LegacyValueExpr{Value: IntSet} }
func (*LegacyBuilder) NatSet() LegacyBuilderExpr     { return LegacyValueExpr{Value: NatSet} }

func (*LegacyBuilder) Declaration(name string, body LegacyBuilderExpr, parameters ...OperatorParam) LegacyOperDecl {
	return LegacyOperDecl{Name: name, FormalParams: append([]OperatorParam(nil), parameters...), Body: body}
}
func (*LegacyBuilder) Operator(operator Operator, arguments ...LegacyBuilderExpr) LegacyBuilderExpr {
	return LegacyOperatorExpr{Operator: operator, Arguments: append([]LegacyBuilderExpr(nil), arguments...)}
}
func (*LegacyBuilder) LetIn(body LegacyBuilderExpr, declarations ...*OperDecl) LegacyBuilderExpr {
	return LegacyLetExpr{Body: body, Declarations: append([]*OperDecl(nil), declarations...)}
}
func (builder *LegacyBuilder) ApplyDeclaration(declaration *OperDecl, arguments ...LegacyBuilderExpr) (LegacyBuilderExpr, error) {
	if declaration == nil {
		return nil, fmt.Errorf("cannot apply a nil declaration")
	}
	if len(arguments) != len(declaration.FormalParams()) {
		return nil, fmt.Errorf("operator %s of %d parameters is applied to %d arguments",
			declaration.Name(), len(declaration.FormalParams()), len(arguments))
	}
	return builder.Operator(Op("OPER_APP"), append([]LegacyBuilderExpr{builder.Name(declaration.Name())}, arguments...)...), nil
}

var legacyBuilderOperatorNameList = []string{
	"EQ", "NE", "OPER_APP", "CHOOSE3", "CHOOSE2",
	"AND", "OR", "NOT", "IMPLIES", "EQUIV", "FORALL3", "EXISTS3", "FORALL2", "EXISTS2",
	"PRIME", "STUTTER", "NO_STUTTER", "ENABLED", "UNCHANGED", "COMPOSE",
	"CASE", "CASE_OTHER", "IF_THEN_ELSE",
	"TEMPORAL_FORALL", "GLOBALLY", "EVENTUALLY", "TEMPORAL_EXISTS", "GUARANTEES", "LEADS_TO", "STRONG_FAIRNESS", "WEAK_FAIRNESS",
	"PLUS", "UNARY_MINUS", "MINUS", "MULT", "DIV", "MOD", "REAL_DIV", "POW", "INT_RANGE", "LT", "GT", "LE", "GE",
	"FiniteSets!Cardinality", "FiniteSets!IsFiniteSet",
	"FUN_APP", "DOMAIN", "RECORD", "EXCEPT", "FUN_CTOR", "TUPLE", "FUN_REC_REF", "FUN_REC_CTOR",
	"Sequences!Append", "Sequences!Concat", "Sequences!Head", "Sequences!Tail", "Sequences!Len", "Sequences!SubSeq",
	"SET_ENUM", "SET_IN", "SET_NOT_IN", "SET_INTERSECT", "SET_UNION2", "SET_FILTER", "FUN_SET", "SET_MAP", "SET_POWERSET", "RECORD_SET", "Sequences!Seq", "SET_MINUS", "SET_SUBSET_EQ", "SET_TIMES", "SET_UNARY_UNION",
	"Apalache!:=", "Apalache!Gen", "Apalache!Skolem", "Apalache!Expand", "Apalache!ConstCardinality", "Apalache!MkSeq", "Apalache!ApaFoldSet", "Apalache!ApaFoldSeqLeft", "Apalache!SetAsFun", "Apalache!Guess",
	"Apalache!Distinct", "Apalache!SelectInSet", "Apalache!SelectInFun", "Apalache!StoreInSet", "Apalache!storeNotInSet", "Apalache!storeNotInFun", "Apalache!UnconstrainArray",
	"Variants!Variant", "Variants!VariantTag", "Variants!VariantGetUnsafe", "Variants!VariantGetOrElse", "Variants!VariantFilter",
}

var legacyBuilderOperators struct {
	sync.Once
	values map[string]Operator
}

func legacyBuilderOperatorRegistry() map[string]Operator {
	legacyBuilderOperators.Do(func() {
		names := legacyBuilderOperatorNameList
		result := make(map[string]Operator, len(names)+2)
		for _, name := range names {
			result[name] = Op(name)
		}
		result["Apalache!SmtMap_AND"] = NewSMTMapOperator(Op("AND"))
		result["Apalache!SmtMap_OR"] = NewSMTMapOperator(Op("OR"))
		legacyBuilderOperators.values = result
	})
	return legacyBuilderOperators.values
}

func (builder *LegacyBuilder) ByName(name string, arguments ...LegacyBuilderExpr) (LegacyBuilderExpr, error) {
	operator, ok := legacyBuilderOperatorRegistry()[name]
	if !ok {
		return nil, NewBuilderError("unknown legacy builder operator " + name)
	}
	return builder.Operator(operator, arguments...), nil
}

func (builder *LegacyBuilder) ByNameOrNull(name string, arguments ...LegacyBuilderExpr) LegacyBuilderExpr {
	operator, ok := legacyBuilderOperatorRegistry()[name]
	if !ok || !operator.IsCorrectArity(len(arguments)) {
		return builder.FromExpr(NullExpr())
	}
	return builder.Operator(operator, arguments...)
}

func BuildLegacyUntyped(block LegacyBuilderExpr) (Expr, error) {
	switch typed := block.(type) {
	case LegacyWrappedExpr:
		if typed.Expr == nil {
			return nil, NewBuilderError("wrapped expression is nil")
		}
		return typed.Expr, nil
	case LegacyNameExpr:
		return NewNameExpr(typed.Name, Untyped()), nil
	case LegacyAlias:
		return BuildLegacyUntyped(typed.Target)
	case LegacyValueExpr:
		return NewValueExpr(typed.Value, Untyped()), nil
	case LegacyOperatorExpr:
		arguments, err := buildLegacyUntypedAll(typed.Arguments)
		if err != nil {
			return nil, err
		}
		return NewOperExpr(typed.Operator, Untyped(), arguments...)
	case LegacyLetExpr:
		body, err := BuildLegacyUntyped(typed.Body)
		if err != nil {
			return nil, err
		}
		return NewLetInExpr(body, Untyped(), typed.Declarations...), nil
	default:
		return nil, NewBuilderError(fmt.Sprintf("unexpected legacy builder expression %T", block))
	}
}

func buildLegacyUntypedAll(blocks []LegacyBuilderExpr) ([]Expr, error) {
	result := make([]Expr, len(blocks))
	for index, block := range blocks {
		var err error
		result[index], err = BuildLegacyUntyped(block)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func BuildLegacyUntypedDeclaration(block LegacyBuilderDecl) (*OperDecl, error) {
	typed, ok := block.(LegacyOperDecl)
	if !ok {
		return nil, NewBuilderError(fmt.Sprintf("unexpected legacy builder declaration %T", block))
	}
	body, err := BuildLegacyUntyped(typed.Body)
	if err != nil {
		return nil, err
	}
	return NewOperDecl(typed.Name, typed.FormalParams, body, Untyped()), nil
}

// BuildLegacyTyped implements TypedPredefs.BuilderExAsTyped. A nil topType
// corresponds to typed() with no explicit annotation.
func BuildLegacyTyped(block LegacyBuilderExpr, aliases map[string]Type, topType Type) (Expr, error) {
	switch typed := block.(type) {
	case LegacyWrappedExpr:
		if typed.Expr == nil {
			return nil, NewBuilderError("wrapped expression is nil")
		}
		if topType != nil {
			return typed.Expr.WithTag(Typed(topType)), nil
		}
		return typed.Expr, nil
	case LegacyNameExpr:
		if topType == nil {
			return nil, NewBuilderError("Missing type annotation in: name " + typed.Name)
		}
		return NewNameExpr(typed.Name, Typed(topType)), nil
	case LegacyAlias:
		aliasType, err := legacyAliasType(aliases, typed.Name)
		if err != nil {
			return nil, err
		}
		return BuildLegacyTyped(typed.Target, aliases, aliasType)
	case LegacyOperatorExpr:
		if topType == nil {
			return nil, NewBuilderError("Missing type annotation in: operator " + typed.Operator.Name())
		}
		arguments := make([]Expr, len(typed.Arguments))
		for index, argument := range typed.Arguments {
			var err error
			arguments[index], err = BuildLegacyTyped(argument, aliases, nil)
			if err != nil {
				return nil, err
			}
		}
		return NewOperExpr(typed.Operator, Typed(topType), arguments...)
	case LegacyLetExpr:
		if topType == nil {
			names := ""
			for _, declaration := range typed.Declarations {
				if declaration != nil {
					names += declaration.Name() + " == ... "
				}
			}
			return nil, NewBuilderError("Missing type annotation in: LET " + names + "in ...")
		}
		body, err := BuildLegacyTyped(typed.Body, aliases, nil)
		if err != nil {
			return nil, err
		}
		return NewLetInExpr(body, Typed(topType), typed.Declarations...), nil
	case LegacyValueExpr:
		return buildLegacyTypedValue(typed.Value, topType)
	default:
		return nil, NewBuilderError(fmt.Sprintf("unexpected legacy builder expression %T", block))
	}
}

func BuildLegacyTypedAlias(block LegacyBuilderExpr, aliases map[string]Type, alias string) (Expr, error) {
	topType, err := legacyAliasType(aliases, alias)
	if err != nil {
		return nil, err
	}
	return BuildLegacyTyped(block, aliases, topType)
}

func buildLegacyTypedValue(value Value, topType Type) (Expr, error) {
	switch value.(type) {
	case IntValue:
		return NewValueExpr(value, Typed(IntT1)), nil
	case BoolValue:
		return NewValueExpr(value, Typed(BoolT1)), nil
	case StringValue:
		if topType == nil {
			topType = StrT1
		}
		return NewValueExpr(value, Typed(topType)), nil
	case PredefinedSet:
		element := map[PredefinedSet]Type{IntSet: IntT1, NatSet: IntT1, BoolSet: BoolT1, RealSet: RealT1, StringSet: StrT1}[value.(PredefinedSet)]
		return NewValueExpr(value, Typed(newSetTypeUnchecked(element))), nil
	default:
		return nil, NewBuilderError(fmt.Sprintf("Unexpected value: %v", value))
	}
}

func legacyAliasType(aliases map[string]Type, alias string) (Type, error) {
	if alias == "?" {
		return nil, NewBuilderError(`An expression is missing a type alias, use tla.foo(...) ? "alias"`)
	}
	if typ, ok := aliases[alias]; ok {
		return typ, nil
	}
	return nil, NewBuilderError("No type given for the type alias " + alias)
}

func BuildLegacyTypedDeclaration(block LegacyOperDecl, aliases map[string]Type, topType Type) (*OperDecl, error) {
	if topType == nil {
		return nil, NewBuilderError("legacy typed declaration requires a top type")
	}
	body, err := BuildLegacyTyped(block.Body, aliases, nil)
	if err != nil {
		return nil, err
	}
	return NewOperDecl(block.Name, block.FormalParams, body, Typed(topType)), nil
}

func BuildLegacyTypedDeclarationAlias(block LegacyOperDecl, aliases map[string]Type, alias string) (*OperDecl, error) {
	topType, ok := aliases[alias]
	if !ok {
		return nil, NewBuilderError("No type for alias " + alias)
	}
	return BuildLegacyTypedDeclaration(block, aliases, topType)
}
