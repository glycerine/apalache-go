package lp

// Source: src/util/lp/stacked_value.h; actual scalar instantiations in lar_solver.h and lar_core_solver.h.

// StackedUint32 adds scoped restoration to the concrete uint32 value.
// Value is directly mutable, matching the source's T& conversion.
// Do not copy after Push: snapshots are owned by this value.
type StackedUint32 struct {
	Value uint32
	stack []uint32
}

func NewStackedUint32(value uint32) StackedUint32 { return StackedUint32{Value: value} }
func (s *StackedUint32) Push()                    { s.stack = append(s.stack, s.Value) }
func (s *StackedUint32) StackSize() uint32        { return uint32(len(s.stack)) }
func (s *StackedUint32) Pop(k uint32) {
	for k > 0 && len(s.stack) > 0 {
		last := len(s.stack) - 1
		s.Value = s.stack[last]
		s.stack = s.stack[:last]
		k--
	}
}

// StackedInt32 adds scoped restoration to the concrete int32 value.
// Value is directly mutable, matching the source's T& conversion.
// Do not copy after Push: snapshots are owned by this value.
type StackedInt32 struct {
	Value int32
	stack []int32
}

func NewStackedInt32(value int32) StackedInt32 { return StackedInt32{Value: value} }
func (s *StackedInt32) Push()                  { s.stack = append(s.stack, s.Value) }
func (s *StackedInt32) StackSize() uint32      { return uint32(len(s.stack)) }
func (s *StackedInt32) Pop(k uint32) {
	for k > 0 && len(s.stack) > 0 {
		last := len(s.stack) - 1
		s.Value = s.stack[last]
		s.stack = s.stack[:last]
		k--
	}
}

// StackedStatus adds scoped restoration to the concrete Status value.
// Value is directly mutable, matching the source's T& conversion.
// Do not copy after Push: snapshots are owned by this value.
type StackedStatus struct {
	Value Status
	stack []Status
}

func NewStackedStatus(value Status) StackedStatus { return StackedStatus{Value: value} }
func (s *StackedStatus) Push()                    { s.stack = append(s.stack, s.Value) }
func (s *StackedStatus) StackSize() uint32        { return uint32(len(s.stack)) }
func (s *StackedStatus) Pop(k uint32) {
	for k > 0 && len(s.stack) > 0 {
		last := len(s.stack) - 1
		s.Value = s.stack[last]
		s.stack = s.stack[:last]
		k--
	}
}

// StackedStrategy adds scoped restoration to the concrete SimplexStrategy value.
// Value is directly mutable, matching the source's T& conversion.
// Do not copy after Push: snapshots are owned by this value.
type StackedStrategy struct {
	Value SimplexStrategy
	stack []SimplexStrategy
}

func NewStackedStrategy(value SimplexStrategy) StackedStrategy { return StackedStrategy{Value: value} }
func (s *StackedStrategy) Push()                               { s.stack = append(s.stack, s.Value) }
func (s *StackedStrategy) StackSize() uint32                   { return uint32(len(s.stack)) }
func (s *StackedStrategy) Pop(k uint32) {
	for k > 0 && len(s.stack) > 0 {
		last := len(s.stack) - 1
		s.Value = s.stack[last]
		s.stack = s.stack[:last]
		k--
	}
}
