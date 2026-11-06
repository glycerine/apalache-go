package numeral

// Source: src/util numeric manager caller/reentry boundary.

// NumericCallerGuard is borrowed from an enclosing source invocation. Bindings
// are caller-owned embedded values; the primitive manager retains only the
// synchronous chain, never a concrete polynomial scope or a global registry.
type NumericCallerGuard interface{ NumericExecuting() bool }
type NumericCallerBinding struct {
	self     *NumericCallerBinding
	previous *NumericCallerBinding
	guard    NumericCallerGuard
	kind     uint32
	integer  *IntegerManager
	natural  *NaturalManager
	modular  *IntegerModularManager
}

func (b *NumericCallerBinding) Previous() *NumericCallerBinding {
	if b == nil {
		return nil
	}
	return b.previous
}
func (b *NumericCallerBinding) Guard() NumericCallerGuard {
	if b == nil {
		return nil
	}
	return b.guard
}
func (m *IntegerManager) CallerBindings() *NumericCallerBinding { return m.callerBindings }
func (m *NaturalManager) CallerBindings() *NumericCallerBinding { return m.callerBindings }
func (m *IntegerManager) BindCaller(b *NumericCallerBinding, g NumericCallerGuard, kind uint32) error {
	if !m.ValidOwner() || b == nil || g == nil || b.self != nil {
		return ErrNumericOwner
	}
	*b = NumericCallerBinding{self: b, previous: m.callerBindings, guard: g, kind: kind, integer: m}
	m.callerBindings = b
	return nil
}
func (m *NaturalManager) BindCaller(b *NumericCallerBinding, g NumericCallerGuard, kind uint32) error {
	if !m.ValidOwner() || b == nil || g == nil || b.self != nil {
		return ErrNumericOwner
	}
	*b = NumericCallerBinding{self: b, previous: m.callerBindings, guard: g, kind: kind, natural: m}
	m.callerBindings = b
	return nil
}
func (b *NumericCallerBinding) Unbind() error {
	if b == nil || b.self != b {
		return ErrNumericOwner
	}
	if b.integer != nil {
		if b.integer.callerBindings != b {
			return ErrNumericOwner
		}
		b.integer.callerBindings = b.previous
	} else if b.natural != nil {
		if b.natural.callerBindings != b {
			return ErrNumericOwner
		}
		b.natural.callerBindings = b.previous
	} else if b.modular != nil {
		if b.modular.callerBindings != b {
			return ErrNumericOwner
		}
		b.modular.callerBindings = b.previous
	} else {
		return ErrNumericOwner
	}
	*b = NumericCallerBinding{}
	return nil
}
func callerBindingsExecuting(b *NumericCallerBinding) bool {
	for ; b != nil; b = b.previous {
		if b.self == b && b.guard != nil && b.guard.NumericExecuting() {
			return true
		}
	}
	return false
}

func (b *NumericCallerBinding) Kind() uint32 {
	if b == nil {
		return 0
	}
	return b.kind
}

func (m *IntegerModularManager) CallerBindings() *NumericCallerBinding { return m.callerBindings }
func (m *IntegerModularManager) BindCaller(b *NumericCallerBinding, g NumericCallerGuard, kind uint32) error {
	if e := m.check(); e != nil {
		return e
	}
	if b == nil || g == nil || b.self != nil {
		return ErrNumericOwner
	}
	*b = NumericCallerBinding{self: b, previous: m.callerBindings, guard: g, kind: kind, modular: m}
	m.callerBindings = b
	return nil
}

// A higher owner reports its actual callback span separately from ordinary
// execution, so literal numeral destructors remain available outside callbacks.
type NumericCallbackGuard interface{ NumericCallbackActive() bool }

func callerBindingsCallbackActive(b *NumericCallerBinding) bool {
	for ; b != nil; b = b.previous {
		if b.self == b && b.guard != nil && b.guard.NumericExecuting() {
			if p, ok := b.guard.(NumericCallbackGuard); ok && p.NumericCallbackActive() {
				return true
			}
		}
	}
	return false
}
