package numeral

// Source: src/util/mpf.h:65-73; src/util/memory_manager.cpp:274-289.

// The backlink is borrowed, lifecycle-only cold metadata installed at the
// actual completed MPQ/Integer member publication. It acquires no value root.
// Ordinary idle retained parents never exclude child use; only the concrete
// synchronous external transport reached by this exact parent does so.
func (z *IntegerManager) mpfLifecycleCallbackActive() bool {
	if z == nil || z.self != z {
		return false
	}
	p := z.MpfParent
	if p == nil || (!p.valueExecuting && !(p.rationalSet.manager == p && p.rationalSet.invocation != 0 && (p.rationalSet.executing || p.rationalSet.finishing)) && !(p.formattedSet.manager == p && p.formattedSet.invocation != 0 && (p.formattedSet.executing || p.formattedSet.finishing))) {
		return false
	}
	return p.self == p && p.integer == z && p.rational != nil && p.rational.self == p.rational && p.rational.Z == z && p.rational.mpfParent == p && p.memory == z.Memory && p.memory != nil && p.memory.FailureTransportActive()
}
func (q *RationalManager) mpfLifecycleCallbackActive() bool {
	return q != nil && q.self == q && q.Z != nil && q.mpfParent != nil && q.mpfParent.rational == q && q.Z.mpfLifecycleCallbackActive()
}
