package ast

// Source: src/ast/bv_decl_plugin.cpp:827-848; src/util/rational.h:519-523.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) BitVectorMultiplicativeInverse(n *unumeral.OwnedRational, bits uint32, out *unumeral.OwnedRational) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.bitVectorMultiplicativeInverse(n, bits, out, nil)
}

// BitVectorMultiplicativeInverseWithInvocation captures this exact inverse
// call before numeric acquisition, for callers with older owning numerals.
func (m *ASTManager) BitVectorMultiplicativeInverseWithInvocation(n *unumeral.OwnedRational, bits uint32, out *unumeral.OwnedRational, c *ASTBitVectorNumeralInvocation) (yes bool, result error) {
	if c == nil {
		return false, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if e := c.prepare(m); e != nil {
		return false, e
	}
	defer c.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.bitVectorMultiplicativeInverseCaptured(n, bits, out, nil, &c.receipt)
}
func (m *ASTManager) bitVectorMultiplicativeInverse(n *unumeral.OwnedRational, bits uint32, out *unumeral.OwnedRational, g *uparams.GlobalParamsService) (bool, error) {
	return m.bitVectorMultiplicativeInverseCaptured(n, bits, out, g, nil)
}
func (m *ASTManager) bitVectorMultiplicativeInverseCaptured(n *unumeral.OwnedRational, bits uint32, out *unumeral.OwnedRational, g *uparams.GlobalParamsService, captured *astBVNormalizationReceipt) (yes bool, result error) {
	q := m.numerals
	one, e := q.IsOne(n)
	if e != nil {
		return false, e
	}
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	f.inverse = true
	if captured != nil {
		*captured = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, captured)
	if one {
		if e = q.CopyWithInvocation(out, n, &f.children[0]); e != nil {
			return false, e
		}
		return true, nil
	}
	// Slots 0/1 hold the parity full-expression, then the correction.
	newValue := func(slot int) error {
		v, e := q.NewValue()
		if e != nil {
			return e
		}
		f.values[slot] = v
		f.placed[slot], f.complete[slot], f.consumed[slot] = true, true, false
		return nil
	}
	takeValue := func(slot int, v unumeral.OwnedRational, e error) error {
		if e != nil {
			return e
		}
		f.values[slot] = v
		f.placed[slot], f.complete[slot], f.consumed[slot] = true, true, false
		return nil
	}
	if e = newValue(0); e != nil {
		return false, e
	}
	f.complete[0] = false // rational(2) completes only after its constructor.
	if e = q.SetInt64WithInvocation(&f.values[0], 2, &f.children[1]); e != nil {
		return false, e
	}
	f.complete[0] = true
	v, e := q.ModWithInvocation(n, &f.values[0], &f.children[2])
	if e = takeValue(1, v, e); e != nil {
		return false, e
	}
	odd, e := q.IsOne(&f.values[1])
	if e != nil {
		return false, e
	}
	for _, slot := range [...]int{1, 0} {
		if e = f.destroyOne(slot, false); e != nil {
			return false, e
		}
	}
	if !odd {
		return false, nil
	}
	// Original default-constructed g, x, y; assignment does not erase prefixes.
	for _, slot := range [...]int{2, 3, 4} {
		if e = newValue(slot); e != nil {
			return false, e
		}
	}
	v, e = q.PowerOfTwoWithInvocation(bits, &f.children[3])
	if e = takeValue(5, v, e); e != nil {
		return false, e
	}
	v, e = q.GCDCoefficientsWithInvocation(n, &f.values[5], &f.values[3], &f.values[4], &f.children[4])
	if e = takeValue(6, v, e); e != nil {
		return false, e
	}
	if e = q.CopyWithInvocation(&f.values[2], &f.values[6], &f.children[5]); e != nil {
		return false, e
	}
	for _, slot := range [...]int{6, 5} {
		if e = f.destroyOne(slot, false); e != nil {
			return false, e
		}
	}
	negative, e := q.IsNeg(&f.values[3])
	if e != nil {
		return false, e
	}
	if negative {
		v, e = q.PowerOfTwoWithInvocation(bits, &f.children[6])
		if e = takeValue(0, v, e); e != nil {
			return false, e
		}
		v, e = q.ModWithInvocation(&f.values[3], &f.values[0], &f.children[7])
		if e = takeValue(1, v, e); e != nil {
			return false, e
		}
		if e = q.CopyWithInvocation(&f.values[3], &f.values[1], &f.children[8]); e != nil {
			return false, e
		}
		for _, slot := range [...]int{1, 0} {
			if e = f.destroyOne(slot, false); e != nil {
				return false, e
			}
		}
	}
	enabled, e := astBVAssertionEnabled(g)
	if e != nil {
		return false, e
	}
	if enabled {
		positive, e := q.IsPos(&f.values[3])
		if e != nil {
			return false, e
		}
		if !positive {
			if e = uparams.NamespaceAssertionFailureOptional(g, "src/ast/bv_decl_plugin.cpp", 844, "x.is_pos()"); e != nil {
				return false, e
			}
		}
	}
	enabled, e = astBVAssertionEnabled(g)
	if e != nil {
		return false, e
	}
	if enabled {
		// Source operand order is unspecified; use power then product explicitly.
		v, e = q.PowerOfTwoWithInvocation(bits, &f.children[9])
		if e = takeValue(5, v, e); e != nil {
			return false, e
		}
		v, e = q.MulWithInvocation(&f.values[3], n, &f.children[10])
		if e = takeValue(6, v, e); e != nil {
			return false, e
		}
		v, e = q.ModWithInvocation(&f.values[6], &f.values[5], &f.children[11])
		if e = takeValue(7, v, e); e != nil {
			return false, e
		}
		valid, e := q.IsOne(&f.values[7])
		if e != nil {
			return false, e
		}
		for _, slot := range [...]int{7, 6, 5} {
			if e = f.destroyOne(slot, false); e != nil {
				return false, e
			}
		}
		if !valid {
			if e = uparams.NamespaceAssertionFailureOptional(g, "src/ast/bv_decl_plugin.cpp", 845, "mod(x * n, rational::power_of_two(bv_size)).is_one()"); e != nil {
				return false, e
			}
		}
	}
	if e = q.CopyWithInvocation(out, &f.values[3], &f.children[12]); e != nil {
		return false, e
	}
	return true, nil
}
