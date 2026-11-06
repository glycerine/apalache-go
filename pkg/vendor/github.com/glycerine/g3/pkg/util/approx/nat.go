package approx

// Source: src/util/approx_nat.h:26-43; src/util/approx_nat.cpp:22-61.

import "strconv"

const natLimit = ^uint32(0) >> 2
const huge = ^uint32(0)

// Nat represents a natural number up to UINT_MAX/4, or the huge sentinel.
// Its zero value is the source default constructor.
type Nat struct{ value uint32 }

func NewNat(value uint32) Nat {
	var n Nat
	n.Set(value)
	return n
}

func (n Nat) IsHuge() bool  { return n.value == huge }
func (n Nat) Value() uint32 { return n.value }

func (n *Nat) Set(value uint32) {
	if value > natLimit {
		n.value = huge
		return
	}
	n.value = value
}

func (n *Nat) AddUint32(value uint32) {
	if n.IsHuge() {
		return
	}
	if value > natLimit || n.value > natLimit-value {
		n.value = huge
		return
	}
	n.value += value
}

func (n *Nat) Add(other Nat) { n.AddUint32(other.value) }

func (n *Nat) MulUint32(value uint32) {
	if n.IsHuge() {
		return
	}
	result := uint64(n.value) * uint64(value)
	if result > uint64(natLimit) {
		n.value = huge
		return
	}
	n.value = uint32(result)
}

func (n *Nat) Mul(other Nat) { n.MulUint32(other.value) }

func (n Nat) LessUint32(value uint32) bool { return !n.IsHuge() && n.value < value }

// Less preserves the source guard on both operands: finite is not less than
// the huge sentinel.
func (n Nat) Less(other Nat) bool {
	return !n.IsHuge() && !other.IsHuge() && n.value < other.value
}

func (n Nat) String() string {
	if n.IsHuge() {
		return "[huge]"
	}
	return strconv.FormatUint(uint64(n.value), 10)
}
