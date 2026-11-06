package ast

// Source: src/ast/bv_decl_plugin.cpp:753-765.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The gate is reached before evaluating the condition. Release erases provider
// access; nil selects the existing enabled-default assertion adapter.
func astBVAssertionEnabled(g *uparams.GlobalParamsService) (bool, error) {
	if !ASTSourceDebug {
		return false, nil
	}
	if g == nil {
		return true, nil
	}
	return g.NamespaceAssertionsEnabled()
}

// NormalizeBitVectorValueWithInvocation returns the original unsigned norm
// value. It does not assign into or copy-normalize the input receiver.
func (m *ASTManager) NormalizeBitVectorValueWithInvocation(value *unumeral.OwnedRational, bits uint32, c *ASTBitVectorNumeralInvocation) (unumeral.OwnedRational, error) {
	return m.normalizeBitVectorValueWithInvocation(value, bits, false, c)
}

// NormalizeSignedBitVectorValueWithInvocation returns the original signed norm.
// Its returned owner stays distinct from the input until caller copy assignment.
func (m *ASTManager) NormalizeSignedBitVectorValueWithInvocation(value *unumeral.OwnedRational, bits uint32, c *ASTBitVectorNumeralInvocation) (unumeral.OwnedRational, error) {
	return m.normalizeBitVectorValueWithInvocation(value, bits, true, c)
}
func (m *ASTManager) normalizeBitVectorValueWithInvocation(value *unumeral.OwnedRational, bits uint32, signed bool, c *ASTBitVectorNumeralInvocation) (out unumeral.OwnedRational, result error) {
	if c == nil {
		return out, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return out, e
	}
	defer m.mu.Unlock()
	if e := c.prepare(m); e != nil {
		return out, e
	}
	defer c.finish(&result)
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.normalizeBitVectorWithFrame(value, bits, signed, nil, &c.receipt)
}

func (m *ASTManager) normalizeBitVector(value *unumeral.OwnedRational, bits uint32, signed bool, g *uparams.GlobalParamsService) (out unumeral.OwnedRational, result error) {
	return m.normalizeBitVectorWithFrame(value, bits, signed, g, nil)
}
func (m *ASTManager) normalizeBitVectorWithFrame(value *unumeral.OwnedRational, bits uint32, signed bool, g *uparams.GlobalParamsService, captured *astBVNormalizationReceipt) (out unumeral.OwnedRational, result error) {
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return out, e
	}
	if captured != nil {
		*captured = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	f.normalization = true
	q := m.numerals
	defer func() {
		defer func() {
			if captured != nil {
				captured.bodyCause = f.bodyCause
				captured.sourceCause = f.sourceCause
				captured.sourceDone = f.sourceDone
				captured.retired = f.retired
			}
		}()
		f.bodyCause = result
		result = errors.Join(result, f.destroy(false))
		if result == nil && f.sourceDone {
			result = f.destroy(true)
			if result == nil {
				for _, i := range f.childOrder() {
					if i >= 0 {
						if e := f.children[i].EndContext(); e != nil {
							result = e
							return
						}
					}
				}
				f.owner.numeralFrames = f.next
				m.BudgetField.Release(f.header)
				f.header = 0
				f.retired = true
			}
		}
	}()
	power := func(slot, child int, width uint32) error {
		v, e := q.PowerOfTwoWithInvocation(width, &f.children[child])
		if e != nil {
			return e
		}
		f.values[slot] = v
		f.placed[slot], f.complete[slot] = true, true
		return nil
	}
	if e = power(1, 0, bits); e != nil {
		return out, e
	}
	f.values[2], e = q.ModWithInvocation(value, &f.values[1], &f.children[1])
	if e != nil {
		return out, e
	}
	f.placed[2], f.complete[2] = true, true
	if e = f.destroyOne(1, false); e != nil {
		return out, e
	}
	enabled, e := astBVAssertionEnabled(g)
	if e != nil {
		return out, e
	}
	if enabled {
		negative, e := q.IsNeg(&f.values[2])
		if e != nil {
			return out, e
		}
		if negative {
			if e = uparams.NamespaceAssertionFailureOptional(g, "src/ast/bv_decl_plugin.cpp", 755, "!r.is_neg()"); e != nil {
				return out, e
			}
		}
	}
	if signed {
		if e = power(3, 3, bits-1); e != nil {
			return out, e
		}
		ge, e := q.GreaterEqual(&f.values[2], &f.values[3])
		if e != nil {
			return out, e
		}
		if e = f.destroyOne(3, false); e != nil {
			return out, e
		}
		if ge {
			if e = power(4, 4, bits); e != nil {
				return out, e
			}
			if e = q.SubAssignWithInvocation(&f.values[2], &f.values[4], &f.children[5]); e != nil {
				return out, e
			}
			if e = f.destroyOne(4, false); e != nil {
				return out, e
			}
		}
		if e = power(5, 6, bits-1); e != nil {
			return out, e
		}
		v, e := q.NewValue()
		if e != nil {
			return out, e
		}
		f.values[6] = v
		f.placed[6] = true
		if e = q.CopyWithInvocation(&f.values[6], &f.values[5], &f.children[7]); e != nil {
			return out, e
		}
		f.complete[6] = true
		if e = q.NegAssign(&f.values[6]); e != nil {
			return out, e
		}
		lt, e := q.Less(&f.values[2], &f.values[6])
		if e != nil {
			return out, e
		}
		if e = f.destroyOne(6, false); e != nil {
			return out, e
		}
		if e = f.destroyOne(5, false); e != nil {
			return out, e
		}
		if lt {
			if e = power(7, 8, bits); e != nil {
				return out, e
			}
			if e = q.AddAssignWithInvocation(&f.values[2], &f.values[7], &f.children[9]); e != nil {
				return out, e
			}
			if e = f.destroyOne(7, false); e != nil {
				return out, e
			}
		}
	}
	out, e = q.NewValue()
	if e != nil {
		return out, e
	}
	if e = q.Move(&out, &f.values[2]); e != nil {
		return out, e
	}
	f.consumed[2] = true
	return out, nil
}

// Captured at the actual normalization frame entry; diagnostic after source end.
// It never authorizes numerical cleanup or adopts another frame generation.
type astBVNormalizationReceipt struct {
	frame                  *astBVNumeralFrame
	invocation             uint64
	sourceDone             bool
	retired                bool
	bodyCause, sourceCause error
}

func (r *astBVNormalizationReceipt) sourceReady(p *astBitVectorPluginOwner) error {
	if r.frame == nil {
		return nil
	}
	if r.sourceDone {
		return r.sourceCause
	}
	f := r.frame
	if f.self != f || f.owner != p || r.invocation == 0 || f.invocation != r.invocation || f.retired {
		return ErrASTHandle
	}
	return r.sourceCause
}

// The captured constructor/normalization frame remains the only cleanup
// authority. Completed returns need no replay of a retired primary owner.
func (r *astBVNormalizationReceipt) cleanup(p *astBitVectorPluginOwner, abort bool) error {
	if r.frame == nil || r.retired {
		return nil
	}
	f := r.frame
	if f.self != f || f.owner != p || f.invocation != r.invocation || f.retired {
		return ErrASTHandle
	}
	return f.destroyPhase(true, abort)
}
