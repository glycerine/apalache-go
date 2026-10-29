package numeral

// Source: src/ast/fpa_decl_plugin.cpp:176-180; src/util/mpf.cpp:316-333.

// CopyFromOwner copies a floating-point value from an explicitly named
// manager. It is the direct-value counterpart of the existing runtime copy
// transport and preserves the exact format, sign, exponent and significand.
func (m *ArbitraryFloatManager) CopyFromOwner(out *ArbitraryFloat, sourceManager *ArbitraryFloatManager, source *ArbitraryFloat) (cause error) {
	if m == nil || sourceManager == nil || m == sourceManager {
		if m == sourceManager && m != nil {
			return m.Copy(out, source)
		}
		return ErrNumericOwner
	}
	if e := m.valueGuard(out); e != nil {
		return e
	}
	if e := sourceManager.valueGuard(source); e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.beginValueCopyBinding(nil, sourceManager.integer); e != nil {
		return e
	}
	defer func() { m.finishValueCopyBinding(cause) }()
	out.format = source.format
	out.exponent = source.exponent
	old := m.valueCopy
	cause = m.integer.copyFromOwnerWithPhysicalFloatCaller(&out.significand, sourceManager.integer, &source.significand, &m.valueCopy, integerAllocationInvocationContext{}, nil, &m.valueBinding)
	m.acknowledgeValueCopyReturn(old)
	return cause
}
