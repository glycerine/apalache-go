package numeral

// Source: mpbq.cpp add/sub and mpbq.h div2, reached by Sturm isolation.
// Integer allocation children stay with the caller's NumericAllocationContext;
// this receipt names only the exact MPBQ diagnostic children of that call.
type BinaryArithmeticInvocation struct {
	self        *BinaryArithmeticInvocation
	manager     *BinaryRationalManager
	debug       binaryDebugChildren
	executing   bool
	sourceCause error
}

func (r *BinaryArithmeticInvocation) valid() bool {
	return r != nil && r.self == r && r.manager != nil && r.manager.self == r.manager
}

func (r *BinaryArithmeticInvocation) Pending() bool {
	return r.valid() && (r.executing || r.debug.pending(r.manager))
}

func (r *BinaryArithmeticInvocation) SourceCause() error {
	if !r.valid() {
		return nil
	}
	if r.executing {
		return ErrNumericPending
	}
	return r.sourceCause
}

func (r *BinaryArithmeticInvocation) Cleanup() error {
	if r == nil || r.self == nil {
		return nil
	}
	if !r.valid() {
		return ErrBinaryOwner
	}
	if r.executing {
		return ErrNumericPending
	}
	return r.debug.cleanup(r.manager)
}

func (r *BinaryArithmeticInvocation) begin(m *BinaryRationalManager) error {
	if r == nil || r.self != nil && r.self != r {
		return ErrBinaryOwner
	}
	if r.Pending() {
		return ErrNumericPending
	}
	if e := m.open(); e != nil {
		return e
	}
	*r = BinaryArithmeticInvocation{manager: m, executing: true}
	r.self = r
	return nil
}

func (r *BinaryArithmeticInvocation) finish(cause error) {
	r.debug.capture(r.manager)
	r.sourceCause = r.debug.sourceCause(r.manager, cause)
	r.executing = false
}

func (o NumericBinaryRationalOperations) AddSubWithInvocation(a, b, out *BinaryRational, subtract bool, r *BinaryArithmeticInvocation) (cause error) {
	if e := o.valid(); e != nil {
		return e
	}
	if e := r.begin(o.manager); e != nil {
		return e
	}
	defer func() { r.finish(cause) }()
	return o.manager.addSubWithAllocationContext(a, b, out, subtract, o.context)
}

func (o NumericBinaryRationalOperations) Div2WithInvocation(a *BinaryRational, r *BinaryArithmeticInvocation) (cause error) {
	if e := o.valid(); e != nil {
		return e
	}
	if e := r.begin(o.manager); e != nil {
		return e
	}
	defer func() { r.finish(cause) }()
	return o.manager.div2WithAllocationContext(a, o.context)
}
