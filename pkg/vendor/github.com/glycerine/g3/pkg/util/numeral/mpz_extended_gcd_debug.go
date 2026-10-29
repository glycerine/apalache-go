//go:build g3debug

package numeral

// Source: src/util/mpz.cpp extended gcd Z3DEBUG diagnostic.

import "github.com/glycerine/g3/pkg/util/diagnostic"

const SourceExtendedGCDDebug = true

// Keep the source action order and reread the actual inputs, including aliases.
// The caller owns nexta/nextb; the actual scalar RAW path reaches end-only DEL.
// Failed scalar bodies retain these locals for explicit owning abort.
func (m *IntegerManager) extendedGCDDiagnostic(r1, r2, a, b, expected, nexta, nextb *Int) error {
	return m.extendedGCDDiagnosticWithAllocationContext(r1, r2, a, b, expected, nexta, nextb, integerAllocationInvocationContext{})
}
func (m *IntegerManager) extendedGCDDiagnosticWithAllocationContext(r1, r2, a, b, expected, nexta, nextb *Int, context integerAllocationInvocationContext) error {
	if e := m.mulWithAllocationContext(a, r1, nexta, context); e != nil {
		return e
	}
	if e := m.mulWithAllocationContext(b, r2, nextb, context); e != nil {
		return e
	}
	if e := m.addSubWithAllocationContext(nexta, nextb, nexta, false, context); e != nil {
		return e
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	comparison, e := m.Compare(nexta, expected)
	if e != nil {
		return e
	}
	if comparison != 0 {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 973, "eq(nexta, tmp1)")
	}
	return nil
}
