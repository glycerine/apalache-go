package apalache

import (
	"fmt"
	"math/big"
)

// Value is a literal or predefined TLA+ value.
type Value interface {
	tlaValue()
}

type IntValue struct{ value *big.Int }

func (IntValue) tlaValue() {}
func NewIntValue(value *big.Int) IntValue {
	if value == nil {
		panic("NewIntValue requires a non-nil integer")
	}
	return IntValue{value: new(big.Int).Set(value)}
}
func Int64Value(value int64) IntValue { return NewIntValue(big.NewInt(value)) }
func (v IntValue) BigInt() *big.Int   { return new(big.Int).Set(v.integer()) }
func (v IntValue) IsNatural() bool    { return v.integer().Sign() >= 0 }
func (v IntValue) String() string     { return v.integer().String() }
func (v IntValue) integer() *big.Int {
	if v.value == nil {
		return new(big.Int)
	}
	return v.value
}

// DecimalValue stores an exact rational parsed from a finite decimal literal.
type DecimalValue struct{ value *big.Rat }

func (DecimalValue) tlaValue() {}
func NewDecimalValue(text string) (DecimalValue, error) {
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return DecimalValue{}, fmt.Errorf("invalid decimal %q", text)
	}
	return DecimalValue{value: value}, nil
}
func (v DecimalValue) Rat() *big.Rat { return new(big.Rat).Set(v.rat()) }
func (v DecimalValue) String() string {
	return v.rat().RatString()
}
func (v DecimalValue) rat() *big.Rat {
	if v.value == nil {
		return new(big.Rat)
	}
	return v.value
}

// RealValue represents the abstract real sort or its Infinity constant. TLA+
// has no efficient concrete representation for arbitrary reals.
type RealValue struct{ infinity bool }

func (RealValue) tlaValue()          {}
func (v RealValue) IsInfinity() bool { return v.infinity }

func Real() RealValue         { return RealValue{} }
func RealInfinity() RealValue { return RealValue{infinity: true} }

type BoolValue bool

func (BoolValue) tlaValue()    {}
func (v BoolValue) Bool() bool { return bool(v) }

type StringValue string

func (StringValue) tlaValue()       {}
func (v StringValue) Value() string { return string(v) }

type PredefinedSet uint8

const (
	BoolSet PredefinedSet = iota
	StringSet
	IntSet
	NatSet
	RealSet
)

func (PredefinedSet) tlaValue() {}
func (s PredefinedSet) Name() string {
	switch s {
	case BoolSet:
		return "BOOLEAN"
	case StringSet:
		return "STRING"
	case IntSet:
		return "Int"
	case NatSet:
		return "Nat"
	case RealSet:
		return "Real"
	default:
		return ""
	}
}

func EqualValues(left, right Value) bool {
	switch l := left.(type) {
	case IntValue:
		r, ok := right.(IntValue)
		return ok && l.integer().Cmp(r.integer()) == 0
	case DecimalValue:
		r, ok := right.(DecimalValue)
		return ok && l.rat().Cmp(r.rat()) == 0
	case RealValue:
		r, ok := right.(RealValue)
		return ok && l.infinity == r.infinity
	case BoolValue:
		r, ok := right.(BoolValue)
		return ok && l == r
	case StringValue:
		r, ok := right.(StringValue)
		return ok && l == r
	case PredefinedSet:
		r, ok := right.(PredefinedSet)
		return ok && l == r
	default:
		return left == nil && right == nil
	}
}
