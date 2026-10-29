package numeral

// Source: src/util numeric primitive caller boundaries.

// NumericCallerPolicy borrows an existing caller-owned value. Interface
// dispatch avoids allocating function closures for warmed source invocations.
type NumericCallerPolicy interface {
	NumericRoots(*IntegerManager, *NumericAllocationRoots) *NumericAllocationRoots
}
type NumericCallerBoundPolicy interface{ NumericBound(bool) bool }

// NumericCallerModularPolicy validates the exact borrowed modular receiver.
type NumericCallerModularPolicy interface {
	NumericModularOwner(*IntegerModularManager) bool
}
type NumericCallerPreparePolicy interface {
	NumericPrepare(*IntegerManager, string) error
}
type NumericCallerRecordPolicy interface {
	NumericRecord(*IntegerManager, string, NumericInvocationReceipt)
}

type NumericCallerScalarPolicy interface {
	NumericScalarRaw(*IntegerManager, string) (*IntegerScalarRawScope, error)
}
type NumericCallerNormalizePolicy interface {
	NumericNormalize(*IntegerModularManager, Int) (Int, error)
}
type NumericCallerSourceCausePolicy interface{ NumericSourceCause(*IntegerManager) error }

func (h *NumericInvocationHooks) callRoots(m *IntegerManager, r *NumericAllocationRoots) *NumericAllocationRoots {
	if h.Policy != nil {
		return h.Policy.NumericRoots(m, r)
	}
	if h.Roots == nil {
		return nil
	}
	return h.Roots(m, r)
}
func (h *NumericInvocationHooks) callBound(executing bool) bool {
	if p, ok := h.Policy.(NumericCallerBoundPolicy); ok {
		return p.NumericBound(executing)
	}
	return h.Bound != nil && h.Bound(executing)
}
func (h *NumericInvocationHooks) callPrepare(m *IntegerManager, phase string) error {
	if p, ok := h.Policy.(NumericCallerPreparePolicy); ok {
		return p.NumericPrepare(m, phase)
	}
	if h.Prepare != nil {
		return h.Prepare(m, phase)
	}
	return nil
}
func (h *NumericInvocationHooks) callRecord(m *IntegerManager, event string, r NumericInvocationReceipt) {
	if p, ok := h.Policy.(NumericCallerRecordPolicy); ok {
		p.NumericRecord(m, event, r)
		return
	}
	if h.Record != nil {
		h.Record(m, event, r)
	}
}

func (h *NumericInvocationHooks) hasScalarRaw() bool {
	_, ok := h.Policy.(NumericCallerScalarPolicy)
	return ok && h.scalarEnabled() || h.ScalarRaw != nil
}
func (h *NumericInvocationHooks) callScalarRaw(m *IntegerManager, event string) (*IntegerScalarRawScope, error) {
	if p, ok := h.Policy.(NumericCallerScalarPolicy); ok && h.scalarEnabled() {
		return p.NumericScalarRaw(m, event)
	}
	return h.ScalarRaw(m, event)
}
func (h *NumericInvocationHooks) hasNormalize() bool {
	_, ok := h.Policy.(NumericCallerNormalizePolicy)
	return ok || h.Normalize != nil
}

// Transport the numeral value across the caller interface. The callback may
// update its cell identity, but cannot retain the address of a lexical holder.
func (h *NumericInvocationHooks) callNormalize(m *IntegerModularManager, a *Int) error {
	if a == nil {
		return ErrNumericOwner
	}
	var value Int
	var err error
	if p, ok := h.Policy.(NumericCallerNormalizePolicy); ok {
		value, err = p.NumericNormalize(m, *a)
	} else {
		value, err = h.Normalize(m, *a)
	}
	*a = value
	return err
}

func (h *NumericInvocationHooks) callSourceCause(m *IntegerManager) error {
	if p, ok := h.Policy.(NumericCallerSourceCausePolicy); ok {
		return p.NumericSourceCause(m)
	}
	if h.SourceCause != nil {
		return h.SourceCause(m)
	}
	return nil
}

// Optional capability predicates select only reached caller specializations.
// Other calls retain the primitive implementation's ordinary lexical scopes.

type NumericCallerScalarEnabledPolicy interface{ NumericScalarRawEnabled() bool }

func (h *NumericInvocationHooks) scalarEnabled() bool {
	if p, ok := h.Policy.(NumericCallerScalarEnabledPolicy); ok {
		return p.NumericScalarRawEnabled()
	}
	return true
}

// NumericPowerInvocation borrows the actual caller-owned RAW power numeral.
type NumericPowerInvocation interface {
	NumericPowerValue() *Int
	NumericPowerFinish() error
	NumericPowerEnd(error)
}
type NumericCallerPowerPolicy interface {
	NumericPowerEnabled() bool
	NumericPower(*IntegerModularManager) (NumericPowerInvocation, error)
}
