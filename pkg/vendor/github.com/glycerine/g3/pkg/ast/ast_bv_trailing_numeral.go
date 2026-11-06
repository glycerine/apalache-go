package ast

// Source: src/ast/rewriter/bv_trailing.cpp:213-234,311-327.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	// beginBitVectorTrailingNumeral guards the allocation-free default rational
	// on the nonnumeral path; matched calls own its copied stored value and two.
)

func (m *ASTManager) beginBitVectorTrailingNumeral(arg ASTHandle, receipt *astBVNormalizationReceipt) (f *astBVNumeralFrame, r astNodeRef, width uint32, matched bool, result error) {
	r, result = m.requireExpr(arg)
	if result != nil {
		return
	}
	matched, result = m.bitVectorAppKind(r, 0)
	if result != nil || !matched {
		return
	}
	f, result = m.beginBVNumeralFrame()
	if result != nil {
		return
	}
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	f.trailingNumeral = true
	q := m.numerals
	f.values[0], result = q.NewValue()
	if result != nil {
		return
	}
	f.placed[0], f.complete[0] = true, true
	_, result = m.readBitVectorNumeralWithInvocation(r, &f.values[0], &width, &f.children[2])
	if result != nil {
		return
	}
	f.values[1], result = q.NewValue()
	if result != nil {
		return
	}
	f.placed[1] = true
	result = q.SetInt64WithInvocation(&f.values[1], 2, &f.children[0])
	if result == nil {
		f.complete[1] = true
	}
	return
}

// bitVectorTrailingDivide preserves the count loop's pre-division increment
// and the removal loop's post-division increment. A successful void call has
// no returned rational; its exact context retires before the next iteration.
func (m *ASTManager) bitVectorTrailingDivide(f *astBVNumeralFrame, limit uint32, before bool) (count uint32, result error) {
	q := m.numerals
	for limit != 0 {
		even, e := q.IsEven(&f.values[0])
		if e != nil {
			return count, e
		}
		if !even {
			break
		}
		if before {
			count++
		}
		if e = q.IDivIntoWithInvocation(&f.values[0], &f.values[1], &f.values[0], &f.children[1]); e != nil {
			return count, e
		}
		if !before {
			count++
		}
		limit--
		if e = f.children[1].SourceReady(); e != nil {
			return count, e
		}
		if e = f.children[1].Cleanup(false); e != nil {
			return count, e
		}
		if e = f.children[1].Cleanup(true); e != nil {
			return count, e
		}
		if e = f.children[1].EndContext(); e != nil {
			return count, e
		}
		// Reuse only a fully retired source call site, never a stopped child.
		f.children[1] = unumeral.RationalServiceInvocation{}
	}
	return count, nil
}

// CountBitVectorNumeralTrailingZeros implements the numeral branch, including
// its width cap. Structural analysis and depth caches belong to the caller.
func (m *ASTManager) CountBitVectorNumeralTrailingZeros(arg ASTHandle) (matched bool, count uint32, result error) {
	return m.countBitVectorNumeralTrailingZeros(arg, nil)
}
func (m *ASTManager) countBitVectorNumeralTrailingZeros(arg ASTHandle, c *ASTBitVectorNumeralInvocation) (matched bool, count uint32, result error) {
	if e := m.lock(); e != nil {
		return false, 0, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return false, 0, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, _, width, matched, e := m.beginBitVectorTrailingNumeral(arg, receipt)
	if f != nil {
		defer f.finishNumeral(&result, receipt)
	}
	if e != nil || !matched {
		return matched, 0, e
	}
	count, result = m.bitVectorTrailingDivide(f, width, true)
	return matched, count, result
}

// RemoveBitVectorNumeralTrailingZeros publishes null for complete removal,
// retains raw identity when no bit was removed, and constructs the reduced
// numeral otherwise. Zero limit and nonnumeral input leave output unread.
func (m *ASTManager) RemoveBitVectorNumeralTrailingZeros(arg ASTHandle, n uint32, out ASTRef) (matched bool, removed uint32, result error) {
	return m.removeBitVectorNumeralTrailingZeros(arg, n, out, nil)
}
func (m *ASTManager) removeBitVectorNumeralTrailingZeros(arg ASTHandle, n uint32, out ASTRef, c *ASTBitVectorNumeralInvocation) (matched bool, removed uint32, result error) {
	if e := m.lock(); e != nil {
		return false, 0, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return false, 0, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	if n == 0 {
		return false, 0, nil
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, r, width, matched, e := m.beginBitVectorTrailingNumeral(arg, receipt)
	if f != nil {
		defer f.finishNumeral(&result, receipt)
	}
	if e != nil || !matched {
		return matched, 0, e
	}
	limit := n
	if width < limit {
		limit = width
	}
	removed, e = m.bitVectorTrailingDivide(f, limit, false)
	if e != nil {
		return true, removed, e
	}
	// two belongs to the called numerical helper, so ends before the caller's
	// reduced numeral construction and expr_ref assignment.
	if e = f.children[0].SourceReady(); e != nil {
		return true, removed, e
	}
	if e = f.destroyOne(1, false); e != nil {
		return true, removed, e
	}
	var node astNodeRef
	newWidth := width - removed
	if newWidth != 0 {
		node = r
		if removed != 0 {
			node, e = m.mkBitVectorNumeralCaptured(&f.values[0], newWidth, &f.normalizationChild)
			if e != nil {
				return true, removed, e
			}
		}
	}
	return true, removed, m.assignRef(out, node)
}
