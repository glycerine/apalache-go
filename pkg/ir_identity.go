package apalache

import (
	"fmt"
	"math"
	"sync/atomic"
)

// UID identifies an IR node. Zero is reserved as NullUID.
type UID struct{ id uint64 }

var nextUID atomic.Uint64

func init() { nextUID.Store(1) }

func NullUID() UID { return UID{} }

// NewUID returns a process-unique identifier. Like Scala UID.unique, it is
// safe for concurrent callers and reserves the signed 64-bit maximum.
func NewUID() UID {
	id := nextUID.Add(1) - 1
	if id >= math.MaxInt64 {
		panic("too many identifiers; change the underlying UID representation")
	}
	return UID{id: id}
}

func (u UID) Uint64() uint64 { return u.id }
func (u UID) String() string { return fmt.Sprintf("%d", u.id) }

func (u UID) Compare(other UID) int {
	switch {
	case u.id < other.id:
		return -1
	case u.id > other.id:
		return 1
	default:
		return 0
	}
}

// TypeTag is either untyped (the zero value) or carries a Type System 1 type.
type TypeTag struct{ typ Type }

func Untyped() TypeTag { return TypeTag{} }
func Typed(typ Type) TypeTag {
	if typ == nil {
		panic("Typed requires a non-nil type")
	}
	return TypeTag{typ: typ}
}
func (tag TypeTag) IsTyped() bool { return tag.typ != nil }
func (tag TypeTag) Type() (Type, bool) {
	return tag.typ, tag.typ != nil
}
func (tag TypeTag) Equal(other TypeTag) bool { return EqualTypes(tag.typ, other.typ) }
func (tag TypeTag) String() string {
	if tag.typ == nil {
		return "Untyped"
	}
	return "Typed(" + tag.typ.String() + ")"
}

// OperatorParam is a value parameter at arity zero and an operator parameter
// at positive arity. Negative arity is retained for Scala compatibility and is
// treated as a value parameter.
type OperatorParam struct {
	Name  string
	Arity int
}

func ValueParam(name string) OperatorParam { return OperatorParam{Name: name} }
func (p OperatorParam) IsOperator() bool   { return p.Arity > 0 }
