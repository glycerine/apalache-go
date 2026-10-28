package apalache

import (
	"fmt"
	"sort"
)

type Interpretation uint8

const (
	InterpretationPredefined Interpretation = iota
	InterpretationStandardLibrary
	InterpretationUser
	InterpretationSignature
)

type OperatorCategory string

const (
	CategoryBase             OperatorCategory = "base"
	CategoryBoolean          OperatorCategory = "boolean"
	CategoryArithmetic       OperatorCategory = "arithmetic"
	CategoryControl          OperatorCategory = "control"
	CategorySet              OperatorCategory = "set"
	CategoryFunction         OperatorCategory = "function"
	CategorySequence         OperatorCategory = "sequence"
	CategoryFiniteSet        OperatorCategory = "finite-set"
	CategoryAction           OperatorCategory = "action"
	CategoryTemporal         OperatorCategory = "temporal"
	CategoryVariant          OperatorCategory = "variant"
	CategoryApalache         OperatorCategory = "apalache"
	CategoryApalacheInternal OperatorCategory = "apalache-internal"
)

type Precedence struct{ Low, High int }

// Arity is an immutable predicate over non-negative argument counts.
type Arity struct{ accepts func(int) bool }

func (a Arity) Accepts(count int) bool { return count >= 0 && a.accepts != nil && a.accepts(count) }
func (a Arity) And(other Arity) Arity {
	return Arity{accepts: func(count int) bool { return a.Accepts(count) && other.Accepts(count) }}
}
func (a Arity) Or(other Arity) Arity {
	return Arity{accepts: func(count int) bool { return a.Accepts(count) || other.Accepts(count) }}
}
func (a Arity) Not() Arity {
	return Arity{accepts: func(count int) bool { return !a.Accepts(count) }}
}
func FixedArity(count int) Arity {
	return Arity{accepts: func(value int) bool { return value == count }}
}
func MinimalArity(count int) Arity {
	return Arity{accepts: func(value int) bool { return value >= count }}
}
func AnyArity() Arity         { return Arity{accepts: func(int) bool { return true }} }
func AnyOddArity() Arity      { return Arity{accepts: func(value int) bool { return value%2 == 1 }} }
func AnyEvenArity() Arity     { return Arity{accepts: func(value int) bool { return value%2 == 0 }} }
func AnyPositiveArity() Arity { return Arity{accepts: func(value int) bool { return value > 0 }} }

type operatorDefinition struct {
	name           string
	interpretation Interpretation
	category       OperatorCategory
	arity          Arity
	precedence     Precedence
	constructible  bool
}

// Operator is immutable metadata for an IR built-in operator.
type Operator struct{ definition *operatorDefinition }

func (o Operator) valid() *operatorDefinition {
	if o.definition == nil {
		panic("zero Operator has no metadata")
	}
	return o.definition
}
func (o Operator) Name() string                   { return o.valid().name }
func (o Operator) String() string                 { return o.Name() }
func (o Operator) Interpretation() Interpretation { return o.valid().interpretation }
func (o Operator) Category() OperatorCategory     { return o.valid().category }
func (o Operator) Precedence() Precedence         { return o.valid().precedence }
func (o Operator) IsCorrectArity(count int) bool  { return o.valid().arity.Accepts(count) }
func (o Operator) Constructible() bool            { return o.valid().constructible }
func (o Operator) Equal(other Operator) bool      { return o.Name() == other.Name() }

var builtinOperators = make(map[string]*operatorDefinition)

func defineOperator(name string, interpretation Interpretation, category OperatorCategory, arity Arity, low, high int) Operator {
	if _, exists := builtinOperators[name]; exists {
		panic("duplicate operator " + name)
	}
	definition := &operatorDefinition{name: name, interpretation: interpretation, category: category, arity: arity, precedence: Precedence{low, high}, constructible: true}
	builtinOperators[name] = definition
	return Operator{definition: definition}
}

func prohibit(operator Operator) Operator {
	operator.valid().constructible = false
	return operator
}

func LookupOperator(name string) (Operator, bool) {
	definition, ok := builtinOperators[name]
	return Operator{definition: definition}, ok
}

func AllOperators() []Operator {
	result := make([]Operator, 0, len(builtinOperators))
	for _, definition := range builtinOperators {
		result = append(result, Operator{definition: definition})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result
}

// NewSMTMapOperator constructs ApalacheInternalOper.smtMap(mapOper).
func NewSMTMapOperator(mapped Operator) Operator {
	return Operator{definition: &operatorDefinition{
		name: "Apalache!SmtMap_" + mapped.Name(), interpretation: InterpretationStandardLibrary,
		category: CategoryApalacheInternal, arity: FixedArity(2), precedence: Precedence{160, 160}, constructible: true,
	}}
}

func mustLookupOperator(name string) Operator {
	operator, ok := LookupOperator(name)
	if !ok {
		panic(fmt.Sprintf("operator %q is not registered", name))
	}
	return operator
}

// Operator accessors retain stable, immutable metadata without exposing
// assignable package globals.
func Op(name string) Operator { return mustLookupOperator(name) }

func init() {
	pre := InterpretationPredefined
	lib := InterpretationStandardLibrary
	fixed := FixedArity
	any := AnyArity()
	positive := AnyPositiveArity()
	odd3 := AnyOddArity().And(MinimalArity(3))
	even2 := AnyEvenArity().And(MinimalArity(2))

	// TlaOper and Boolean operators.
	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"EQ", fixed(2), 50, 50}, {"NE", fixed(2), 50, 50}, {"OPER_APP", positive, 160, 160},
		{"CHOOSE3", fixed(3), 0, 0}, {"CHOOSE2", fixed(2), 0, 0}, {"LABEL", MinimalArity(2), 0, 0},
		{"AND", any, 30, 30}, {"OR", any, 30, 30}, {"NOT", fixed(1), 40, 40},
		{"IMPLIES", fixed(2), 10, 10}, {"EQUIV", fixed(2), 20, 20}, {"FORALL3", fixed(3), 0, 0},
		{"FORALL2", fixed(2), 0, 0}, {"EXISTS3", fixed(3), 0, 0}, {"EXISTS2", fixed(2), 0, 0},
	} {
		category := CategoryBase
		if spec.name == "AND" || spec.name == "OR" || spec.name == "NOT" || spec.name == "IMPLIES" || spec.name == "EQUIV" || spec.name == "FORALL3" || spec.name == "FORALL2" || spec.name == "EXISTS3" || spec.name == "EXISTS2" {
			category = CategoryBoolean
		}
		defineOperator(spec.name, pre, category, spec.arity, spec.low, spec.high)
	}

	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"PLUS", fixed(2), 100, 100}, {"UNARY_MINUS", fixed(1), 120, 120}, {"MINUS", fixed(2), 110, 110},
		{"MULT", fixed(2), 130, 130}, {"DIV", fixed(2), 130, 130}, {"MOD", fixed(2), 100, 110},
		{"REAL_DIV", fixed(2), 130, 130}, {"POW", fixed(2), 140, 140}, {"INT_RANGE", fixed(2), 90, 90},
		{"LT", fixed(2), 50, 50}, {"GT", fixed(2), 50, 50}, {"LE", fixed(2), 50, 50}, {"GE", fixed(2), 50, 50},
	} {
		defineOperator(spec.name, lib, CategoryArithmetic, spec.arity, spec.low, spec.high)
	}

	defineOperator("CASE", pre, CategoryControl, even2, 0, 0)
	defineOperator("CASE_OTHER", pre, CategoryControl, odd3, 0, 0)
	defineOperator("IF_THEN_ELSE", pre, CategoryControl, fixed(3), 0, 0)

	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"SET_ENUM", any, 160, 160}, {"FUN_SET", fixed(2), 160, 160}, {"RECORD_SET", even2, 160, 160},
		{"Sequences!Seq", fixed(1), 160, 160}, {"SET_IN", fixed(2), 50, 50}, {"SET_NOT_IN", fixed(2), 50, 50},
		{"SET_UNION2", fixed(2), 80, 80}, {"SET_INTERSECT", fixed(2), 80, 80}, {"SET_SUBSET_EQ", fixed(2), 50, 50},
		{"SET_MINUS", fixed(2), 80, 80}, {"SET_FILTER", fixed(3), 160, 160}, {"SET_MAP", odd3, 160, 160},
		{"SET_POWERSET", fixed(1), 100, 130}, {"SET_UNARY_UNION", fixed(1), 100, 130}, {"SET_TIMES", MinimalArity(2), 100, 130},
	} {
		defineOperator(spec.name, pre, CategorySet, spec.arity, spec.low, spec.high)
	}

	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"RECORD", AnyEvenArity(), 160, 160}, {"TUPLE", any, 160, 160}, {"FUN_APP", fixed(2), 160, 160},
		{"DOMAIN", fixed(1), 100, 130}, {"FUN_CTOR", odd3, 160, 160}, {"FUN_REC_CTOR", MinimalArity(3), 160, 160},
		{"FUN_REC_REF", fixed(0), 160, 160}, {"EXCEPT", odd3, 160, 160},
	} {
		defineOperator(spec.name, pre, CategoryFunction, spec.arity, spec.low, spec.high)
	}

	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"Sequences!Head", fixed(1), 160, 160}, {"Sequences!Tail", fixed(1), 160, 160},
		{"Sequences!Append", fixed(2), 160, 160}, {"Sequences!Concat", fixed(2), 130, 130},
		{"Sequences!Len", fixed(1), 160, 160}, {"Sequences!SubSeq", fixed(3), 160, 160},
		{"SequencesExt!ToSet", fixed(1), 160, 160},
	} {
		defineOperator(spec.name, lib, CategorySequence, spec.arity, spec.low, spec.high)
	}
	prohibit(defineOperator("Sequences!SelectSeq", lib, CategorySequence, fixed(2), 160, 160))

	defineOperator("FiniteSets!IsFiniteSet", lib, CategoryFiniteSet, fixed(1), 160, 160)
	defineOperator("FiniteSets!Cardinality", lib, CategoryFiniteSet, fixed(1), 160, 160)

	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"PRIME", fixed(1), 150, 150}, {"STUTTER", fixed(2), 40, 150}, {"NO_STUTTER", fixed(2), 40, 150},
		{"ENABLED", fixed(1), 40, 150}, {"UNCHANGED", fixed(1), 40, 150}, {"COMPOSE", fixed(2), 50, 140},
	} {
		defineOperator(spec.name, pre, CategoryAction, spec.arity, spec.low, spec.high)
	}

	for _, spec := range []struct {
		name      string
		arity     Arity
		low, high int
	}{
		{"GLOBALLY", fixed(1), 40, 150}, {"EVENTUALLY", fixed(1), 40, 150}, {"LEADS_TO", fixed(2), 20, 20},
		{"GUARANTEES", fixed(2), 20, 20}, {"WEAK_FAIRNESS", fixed(2), 40, 150}, {"STRONG_FAIRNESS", fixed(2), 40, 150},
		{"TEMPORAL_EXISTS", fixed(2), 0, 0}, {"TEMPORAL_FORALL", fixed(2), 0, 0},
	} {
		defineOperator(spec.name, pre, CategoryTemporal, spec.arity, spec.low, spec.high)
	}

	for _, spec := range []struct {
		name  string
		arity int
	}{
		{"Variants!Variant", 2}, {"Variants!VariantFilter", 2}, {"Variants!VariantTag", 1},
		{"Variants!VariantGetOrElse", 3}, {"Variants!VariantGetUnsafe", 2},
	} {
		defineOperator(spec.name, lib, CategoryVariant, fixed(spec.arity), 160, 160)
	}

	for _, spec := range []struct {
		name           string
		arity          int
		interpretation Interpretation
		low, high      int
	}{
		{"Apalache!<:", 2, lib, 70, 70}, {"Apalache!:=", 2, lib, 50, 50}, {"Apalache!Gen", 1, lib, 160, 160},
		{"Apalache!Skolem", 1, lib, 160, 160}, {"Apalache!Guess", 1, pre, 160, 160},
		{"Apalache!Expand", 1, lib, 160, 160}, {"Apalache!ConstCardinality", 1, lib, 160, 160},
		{"Apalache!MkSeq", 2, lib, 160, 160}, {"Apalache!ApaFoldSet", 3, lib, 160, 160},
		{"Apalache!ApaFoldSeqLeft", 3, lib, 160, 160}, {"Apalache!Repeat", 3, lib, 160, 160},
		{"Apalache!SetAsFun", 1, lib, 160, 160},
	} {
		defineOperator(spec.name, spec.interpretation, CategoryApalache, fixed(spec.arity), spec.low, spec.high)
	}
	prohibit(defineOperator("Apalache!FunAsSeq", lib, CategoryApalache, fixed(2), 160, 160))

	for _, spec := range []struct {
		name  string
		arity Arity
	}{
		{"ApalacheInternal!__NotSupportedByModelChecker", fixed(1)}, {"Apalache!Distinct", any},
		{"TLC!TLCGet", fixed(1)}, {"TLC!ToString", fixed(1)},
		{"ApalacheInternal!__ApalacheSeqCapacity", fixed(1)}, {"Apalache!SelectInSet", fixed(2)},
		{"Apalache!SelectInFun", fixed(2)}, {"Apalache!StoreInSet", fixed(2).Or(fixed(3))},
		{"Apalache!storeNotInSet", fixed(2)}, {"Apalache!storeNotInFun", fixed(2)},
		{"Apalache!UnconstrainArray", fixed(1)},
	} {
		defineOperator(spec.name, lib, CategoryApalacheInternal, spec.arity, 160, 160)
	}
}
