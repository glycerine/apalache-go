package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2141-2168.

// BitVectorZeroBit follows the original numeral division/remainder and
// reverse concat traversal. Unknown bits return false without numerical
// owners; numeral bits read the stored value without width normalization.
func (m *ASTManager) BitVectorZeroBit(arg ASTHandle, idx uint32) (zero bool, result error) {
	return m.bitVectorZeroBit(arg, idx, nil)
}

// BitVectorZeroBitWithInvocation names this exact helper call for a caller
// whose older numerical/reference locals must join it before source DT.
func (m *ASTManager) BitVectorZeroBitWithInvocation(arg ASTHandle, idx uint32, c *ASTBitVectorNumeralInvocation) (bool, error) {
	if c == nil {
		return false, ErrASTHandle
	}
	return m.bitVectorZeroBit(arg, idx, c)
}
func (m *ASTManager) bitVectorZeroBit(arg ASTHandle, idx uint32, c *ASTBitVectorNumeralInvocation) (zero bool, result error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return false, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	r, e := m.requireExpr(arg)
	if e != nil {
		return false, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	for {
		numeral, e := m.bitVectorAppKind(r, 0)
		if e != nil {
			return false, e
		}
		if numeral {
			return m.bitVectorNumeralZeroBit(r, idx, receipt)
		}
		concat, e := m.bitVectorAppKind(r, 37)
		if e != nil {
			return false, e
		}
		if !concat {
			return false, nil
		}
		a := m.appAt(r.slot)
		found := false
		for i := a.count; i != 0; {
			i--
			child, e := m.readInternal(a.args, i)
			if e != nil {
				return false, e
			}
			sort, e := m.sortOf(child)
			if e != nil {
				return false, e
			}
			width, e := m.bitVectorSortSize(sort, nil)
			if e != nil {
				return false, e
			}
			if width <= idx {
				idx -= width
			} else {
				r = child
				found = true
				break
			}
		}
		if !found {
			// Source UNREACHABLE is undefined in release; reject that domain.
			return false, ErrASTDomain
		}
	}
}

func (m *ASTManager) bitVectorNumeralZeroBit(r astNodeRef, idx uint32, receipt *astBVNormalizationReceipt) (zero bool, result error) {
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	f.zeroBit = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	q := m.numerals
	f.values[0], e = q.NewValue()
	if e != nil {
		return false, e
	}
	f.placed[0], f.complete[0] = true, true
	var width uint32
	_, e = m.readBitVectorNumeralWithInvocation(r, &f.values[0], &width, &f.children[2])
	if e != nil {
		return false, e
	}
	zero, e = q.IsZero(&f.values[0])
	if e != nil || zero {
		return zero, e
	}
	f.values[1], e = q.PowerOfTwoWithInvocation(idx, &f.children[0])
	if e != nil {
		return false, e
	}
	f.placed[1], f.complete[1] = true, true
	if e = q.IDivIntoWithInvocation(&f.values[0], &f.values[1], &f.values[0], &f.children[1]); e != nil {
		return false, e
	}
	if e = f.children[1].SourceReady(); e != nil {
		return false, e
	}
	if e = f.children[0].SourceReady(); e != nil {
		return false, e
	}
	// The power return temporary ends at the void div full expression,
	// before constructing the two/modulo return-expression temporaries.
	if e = f.destroyOne(1, false); e != nil {
		return false, e
	}
	f.values[2], e = q.NewValue()
	if e != nil {
		return false, e
	}
	f.placed[2] = true
	if e = q.SetInt64WithInvocation(&f.values[2], 2, &f.children[3]); e != nil {
		return false, e
	}
	f.complete[2] = true
	f.values[3], e = q.ModWithInvocation(&f.values[0], &f.values[2], &f.children[4])
	if e != nil {
		return false, e
	}
	f.placed[3], f.complete[3] = true, true
	return q.IsZero(&f.values[3])
}
