package numeral

// Source: src/util/mpq.cpp:156-185.

import

// These are the four literal RAW declarations. Failure omits the end label's
// DEL statements; explicit owning cleanup has its own cursor after older NUMs.
umemory "github.com/glycerine/g3/pkg/util/memory"

type rationalDecimalScope struct {
	originalPending umemory.Block
	originalCell    SourceCellBacking
	cellRetired     bool
	Transferred     umemory.SmallAllocatorPendingAllocation

	Parent                                 *ArbitraryFloatFormatInvocation
	Values                                 [4]Int // ten,n1,d1,v1: source end-label DEL order
	Count                                  uint8
	executing, bodySucceeded, ended, owned bool
	cursor, ownCursor                      uint8
	Numeric                                NumericAllocationRoots
	division                               integerRawInvocationReceipt
	insertion                              NumeralInsertionReceipt
	bodyCause, sourceCause, ownCause       error
}

func (m *ArbitraryFloatManager) displayFormatDecimal(s *ArbitraryFloatFormatInvocation, out RationalSourceStream, a *Rational, prec uint32) (err error) {
	d := &s.Decimal
	if d.Parent != nil {
		return ErrNumericPending
	}
	*d = rationalDecimalScope{Parent: s, executing: true, Count: 3}
	context := s.allocationContext()
	context.formatDecimal = d
	z := m.integer
	n1, d1, v1, ten := &d.Values[1], &d.Values[2], &d.Values[3], &d.Values[0]
	defer func() {
		d.executing = false
		d.bodyCause = err
		if err == nil {
			d.bodySucceeded = true
		}
		err = JoinNumericErrors(err, m.finishFormatDecimal(d))
	}()
	if e := z.copyFromOwnerWithAllocationContext(n1, z, &a.Num, nil, context); e != nil {
		return e
	}
	if e := z.copyFromOwnerWithAllocationContext(d1, z, &a.Den, nil, context); e != nil {
		return e
	}
	if a.Num.Val < 0 {
		if e := out.AppendCString("-"); e != nil {
			return e
		}
		if e := z.Neg(n1); e != nil {
			return e
		}
	}
	*ten = NewInt(10)
	d.Count = 4
	if e := z.divWithAllocationContext(n1, d1, v1, context); e != nil {
		return e
	}
	if e := z.insertSourceIntegerTracked(out, v1, nil, &d.insertion); e != nil {
		return e
	}
	if e := z.remWithAllocationContext(n1, d1, n1, context); e != nil {
		return e
	}
	if n1.Val == 0 {
		return nil
	}
	if e := out.AppendCString("."); e != nil {
		return e
	}
	for i := uint32(0); i < prec; i++ {
		if e := z.mulWithAllocationContext(n1, ten, n1, context); e != nil {
			return e
		}
		if e := z.divWithAllocationContext(n1, d1, v1, context); e != nil {
			return e
		}
		if modularSourceDebug {
			less, e := z.Less(v1, ten)
			if e != nil {
				return e
			}
			if !less {
				return ErrIntegerAssertion
			}
		}
		if e := z.insertSourceIntegerTracked(out, v1, nil, &d.insertion); e != nil {
			return e
		}
		if e := z.remWithAllocationContext(n1, d1, n1, context); e != nil {
			return e
		}
		if n1.Val == 0 {
			return nil
		}
	}
	return out.AppendCString("?")
}
func (m *ArbitraryFloatManager) finishFormatDecimal(d *rationalDecimalScope) error {
	if d.Parent == nil {
		return nil
	}
	if !m.MatchesFormatFrame(d.Parent) || d != &d.Parent.Decimal {
		return ErrNumericOwner
	}
	if d.sourceCause != nil {
		return d.sourceCause
	}
	if e := m.integer.RawInvocationSourceCause(&d.division); e != nil {
		d.sourceCause = e
		return e
	}

	if d.insertion.sourceCause != nil {
		d.sourceCause = d.insertion.sourceCause
		return d.sourceCause
	}
	if !d.bodySucceeded || d.ended {
		return nil
	}
	for d.cursor < 4 {
		// ten was constructed after the first three declarations.
		if d.cursor == 0 && d.Count < 4 {
			d.cursor++
			continue
		}
		_, e := SourceNumeralDel(m.integer, &d.Values[d.cursor])
		if e != nil {
			d.sourceCause = e
			return e
		}
		d.cursor++
	}
	d.ended = true
	return nil
}
func (m *ArbitraryFloatManager) cleanupFormatDecimal(d *rationalDecimalScope, abort bool) error {
	if d.Parent == nil {
		return nil
	}
	if !m.MatchesFormatFrame(d.Parent) || d != &d.Parent.Decimal {
		return ErrNumericOwner
	}
	if d.executing {
		return ErrNumericPending
	}
	if !abort {
		if e := m.cleanupFormatInsertion(&d.insertion, false); e != nil {
			return e
		}

		return m.integer.CleanupRawInvocation(&d.division, false)
	}
	for d.ownCursor < 4 {
		if d.ownCursor != 0 || d.Count >= 4 {
			if e := m.integer.Del(&d.Values[d.ownCursor]); e != nil {
				if d.ownCause == nil {
					d.ownCause = e
				}
				return e
			}
		}
		d.ownCursor++
	}
	if e := m.cleanupFormatInsertion(&d.insertion, true); e != nil {
		return e
	}
	if e := m.integer.CleanupRawInvocation(&d.division, true); e != nil {
		return e
	}

	if e := m.integer.cleanupFormatAllocation(&d.Numeric, &d.Transferred, &d.cellRetired); e != nil {
		return e
	}
	d.owned = true
	return nil
}

// Cleanup is bound to the actually issued character/formatting tuples. A later
// unrelated insertion is never adopted through the Natural member address.
func (m *ArbitraryFloatManager) cleanupFormatInsertion(r *NumeralInsertionReceipt, abort bool) error {
	if r == nil || !r.active {
		return nil
	}
	n := m.integer.Natural
	if r.natural != n || r.Character != &n.CharFormatting || r.invocation == 0 {
		return ErrNumericOwner
	}
	if r.characterOwned && r.invocation != r.Character.invocation {
		return ErrNumericOwner
	}
	if r.formattingOwned && (r.formatting != &n.Formatting || r.formattingInvocation != r.formatting.invocation) {
		return ErrNumericOwner
	}
	stop := func(e error) error {
		if e != nil && r.ownCause == nil {
			r.ownCause = e
		}
		return e
	}
	if !r.sourceDone {
		if r.formattingOwned {
			if e := n.unwindFormattingScope(r.formatting); e != nil {
				return stop(e)
			}
		}
		if r.characterOwned && r.characterCompleted {
			if e := n.destroyIntegerCharacters(); e != nil {
				return stop(e)
			}
			r.characterOwned = false
		}
		r.sourceDone = true
	}
	if !abort {
		return nil
	}
	if r.formattingOwned {
		if e := n.cleanupFormattingScope(r.formatting); e != nil {
			return stop(e)
		}
	}
	if r.characterOwned {
		r.Character.Buffer = nil
		n.CharFormatting = integerCharacterScope{invocation: r.invocation}
	}
	r.characterOwned = false
	r.formattingOwned = false
	r.active = false
	r.retired = true
	return nil
}
