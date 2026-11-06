package numeral

// Source: src/util/mpq.cpp:130-139, raw manager display.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (m *RationalManager) DisplayIntoTracked(out RationalSourceStream, a *Rational, receipt *NumeralInsertionReceipt) error {
	if out == nil {
		return arena.ErrConfig
	}
	integer, e := m.IsInt(a)
	if e != nil {
		return e
	}
	if e = m.Z.InsertSourceIntegerTracked(out, &a.Num, receipt, nil); e != nil {
		return e
	}
	if integer {
		return nil
	}
	if e = out.AppendCString("/"); e != nil {
		return e
	}
	return m.Z.InsertSourceIntegerTracked(out, &a.Den, receipt, nil)
}

func (m *RationalManager) CleanupSourceDisplay(receipt *NumeralInsertionReceipt) error {
	if m == nil || m.self != m || m.Z == nil {
		return umemory.ErrRuntimeOwner
	}
	if receipt == nil || !receipt.Active() {
		return nil
	}
	return cleanupOwnedNumeralInsertion(receipt, m.Z.Naturals())
}
