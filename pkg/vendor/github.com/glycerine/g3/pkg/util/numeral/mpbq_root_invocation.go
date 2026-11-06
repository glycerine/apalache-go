package numeral

// Source: mpbq.h root_lower/root_upper SET then mpbq.cpp:261-303.

// BinaryRootInvocation retains the actual numerator copy and integer root.
// It neither owns the output nor authorizes cleanup of a replacement call.
type BinaryRootInvocation struct {
	debug       binaryDebugChildren
	sourceCause error
	self        *BinaryRootInvocation
	manager     *BinaryRationalManager
	copy        IntegerCopyAllocationReceipt
	root        IntegerRootInvocationReceipt
	executing   bool
}

func (r *BinaryRootInvocation) valid() bool {
	return r != nil && r.self == r && r.manager != nil && r.manager.self == r.manager
}
func (r *BinaryRootInvocation) Pending() bool {
	return r.valid() && (r.executing || r.manager.Z.MatchesCopyAllocationInvocation(&r.copy) || r.manager.Z.MatchesRootInvocation(&r.root) || r.debug.pending(r.manager))
}
func (r *BinaryRootInvocation) SourceCause() error {
	if !r.valid() {
		return nil
	}
	if r.executing {
		return ErrRootPending
	}
	if r.sourceCause != nil {
		return r.sourceCause
	}
	return r.manager.Z.RootInvocationSourceCause(&r.root)
}
func (r *BinaryRootInvocation) Cleanup(abort bool) error {
	if r == nil || r.self == nil {
		return nil
	}
	if !r.valid() {
		return ErrBinaryOwner
	}
	if r.executing {
		return ErrRootPending
	}
	if e := r.debug.cleanup(r.manager); e != nil {
		return e
	}
	if e := r.manager.Z.CleanupRootInvocation(&r.root, abort); e != nil {
		return e
	}
	if abort {
		return r.manager.Z.CleanupCopyAllocationInvocation(&r.copy)
	}
	return nil
}
func (m *BinaryRationalManager) RootLowerWithInvocation(a *BinaryRational, n uint32, out *BinaryRational, r *BinaryRootInvocation) (bool, error) {
	return m.rootWithInvocation(a, n, out, true, r)
}
func (m *BinaryRationalManager) RootUpperWithInvocation(a *BinaryRational, n uint32, out *BinaryRational, r *BinaryRootInvocation) (bool, error) {
	return m.rootWithInvocation(a, n, out, false, r)
}
func (m *BinaryRationalManager) rootWithInvocation(a *BinaryRational, n uint32, out *BinaryRational, lower bool, r *BinaryRootInvocation) (exact bool, cause error) {
	if r != nil {
		if r.self != nil && r.self != r {
			return false, ErrBinaryOwner
		}
		if r.Pending() {
			return false, ErrRootPending
		}
	}
	if e := m.pair(out, a); e != nil {
		return false, e
	}
	var copied *IntegerCopyAllocationReceipt
	var rooted *IntegerRootInvocationReceipt
	if r != nil {
		*r = BinaryRootInvocation{manager: m, executing: true}
		r.self = r
		copied, rooted = &r.copy, &r.root
		defer func() {
			r.debug.capture(m)
			r.sourceCause = r.debug.sourceCause(m, cause)
			r.executing = false
		}()
	}
	if e := m.copyWithAllocationReceipt(out, a, copied); e != nil {
		return false, e
	}
	return m.rootWithReceipt(out, n, lower, rooted)
}
