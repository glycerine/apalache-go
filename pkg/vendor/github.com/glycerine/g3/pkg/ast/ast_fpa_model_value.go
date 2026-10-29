package ast

// Source: src/smt/theory_fpa.cpp:128-192,203-232. The concrete bridge keeps
// source numerical operations on the AST plugin's actual MPF/MPZ owners.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func fpaSourceAssert(file string, line int32, condition string, evaluate func() (bool, error)) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	ok, err := evaluate()
	if err != nil || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, file, line, condition)
}

func fpaModelAssert(line int32, condition string, evaluate func() (bool, error)) error {
	return fpaSourceAssert("src/smt/theory_fpa.cpp", line, condition, evaluate)
}

// MkFPAModelValue reconstructs a canonical numeral from the model procedure's
// one packed dependency or three sign/exponent/significand dependencies.
func (m *ASTManager) MkFPAModelValue(ebits, sbits uint32, values []ASTHandle) (out ASTHandle, result error) {
	if err := m.lock(); err != nil {
		return out, err
	}
	defer m.mu.Unlock()
	defer func() {
		if result != nil {
			out = ASTHandle{}
		}
	}()
	if ebits < 2 || sbits < 2 || uint64(ebits)+uint64(sbits) > uint64(^uint32(0)) {
		return out, ErrASTDomain
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded || m.bitVectorPlugin == nil {
		return out, ErrASTHandle
	}
	im, err := p.numeric.BorrowIntegerManager()
	if err != nil {
		return out, err
	}
	var bias unumeral.Int
	defer func() { result = errors.Join(result, im.Del(&bias)) }()
	two := unumeral.NewInt(2)
	if result = im.Power(&two, ebits-1, &bias); result != nil {
		return out, result
	}
	if result = im.Dec(&bias); result != nil {
		return out, result
	}
	var sign, significand, exponent unumeral.Int
	defer func() {
		result = errors.Join(result, im.Del(&exponent))
		result = errors.Join(result, im.Del(&significand))
		result = errors.Join(result, im.Del(&sign))
	}()
	if len(values) == 1 {
		ref, err := m.requireExpr(values[0])
		if err != nil {
			return out, err
		}
		sort, err := m.sortOf(ref)
		if err != nil {
			return out, err
		}
		matched, err := m.bitVectorSortRecognized(sort)
		if err != nil {
			return out, err
		}
		if result = fpaModelAssert(146, "m_bu.is_bv(values[0])", func() (bool, error) { return matched, nil }); result != nil {
			return out, result
		}
		if result = fpaModelAssert(147, "m_bu.get_bv_size(values[0]) == (m_ebits + m_sbits)", func() (bool, error) {
			width, err := m.bitVectorSortSize(sort, nil)
			return width == ebits+sbits, err
		}); result != nil {
			return out, result
		}
		result = func() (err error) {
			number, err := m.numerals.NewValue()
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, m.numerals.Del(&number)) }()
			var packed unumeral.Int
			defer func() { err = errors.Join(err, im.Del(&packed)) }()
			var numeralWidth uint32
			numeral, err := m.readBitVectorNumeral(ref, &number, &numeralWidth)
			if err != nil {
				return err
			}
			if !numeral {
				if err = diagnostic.BorrowProcessDebugContext().VerifyFailure("src/smt/theory_fpa.cpp", 152, "m_bu.is_numeral(values[0], all_r, bv_sz)"); err != nil {
					return err
				}
			}
			if err = fpaModelAssert(153, "bv_sz == (m_ebits + m_sbits)", func() (bool, error) {
				// The failed probe never initialized bv_sz. Do not fabricate a width
				// if a continued VERIFY subsequently reaches this assertion operand.
				if !numeral {
					return false, ErrASTDomain
				}
				return numeralWidth == ebits+sbits, nil
			}); err != nil {
				return err
			}
			if err = fpaModelAssert(154, "all_r.is_int()", func() (bool, error) { return m.numerals.IsInt(&number) }); err != nil {
				return err
			}
			if err = im.CopyFromOwner(&packed, m.numerals.Manager.Z, &number.Raw.Num); err != nil {
				return err
			}
			if err = im.MachineDiv2k(&packed, ebits+sbits-1, &sign); err != nil {
				return err
			}
			power, err := p.numeric.BorrowPowerOfTwo(ebits + sbits - 1)
			if err != nil {
				return err
			}
			if err = im.Mod(&packed, &power, &packed); err != nil {
				return err
			}
			if err = im.MachineDiv2k(&packed, sbits-1, &exponent); err != nil {
				return err
			}
			power, err = p.numeric.BorrowPowerOfTwo(sbits - 1)
			if err != nil {
				return err
			}
			if err = im.Mod(&packed, &power, &packed); err != nil {
				return err
			}
			return im.CopyFromOwner(&significand, im, &packed)
		}()
		if result != nil {
			return out, result
		}
	} else if len(values) == 3 {
		result = func() (err error) {
			var fields [3]unumeral.OwnedRational
			for i := range fields {
				fields[i], err = m.numerals.NewValue()
				if err != nil {
					return err
				}
				defer func(i int) { err = errors.Join(err, m.numerals.Del(&fields[i])) }(i)
			}
			for i, expected := range []uint32{1, ebits, sbits - 1} {
				ref, e := m.requireExpr(values[i])
				if e != nil {
					return e
				}
				var width uint32
				ok, e := m.readBitVectorNumeral(ref, &fields[i], &width)
				if e != nil {
					return e
				}
				conditions := []string{"r && bv_sz == 1", "r && bv_sz == m_ebits", "r && bv_sz == m_sbits - 1"}
				if err = fpaModelAssert(int32(169+2*i), conditions[i], func() (bool, error) { return ok && width == expected, nil }); err != nil {
					return err
				}
			}
			conditions := []string{"mpzm.is_one(sgn_r.to_mpq().denominator())", "mpzm.is_one(exp_r.to_mpq().denominator())", "mpzm.is_one(sig_r.to_mpq().denominator())"}
			for i := range fields {
				if err = fpaModelAssert(int32(176+i), conditions[i], func() (bool, error) { return im.IsOne(&fields[i].Raw.Den) }); err != nil {
					return err
				}
			}
			if err = im.CopyFromOwner(&sign, m.numerals.Manager.Z, &fields[0].Raw.Num); err != nil {
				return err
			}
			if err = im.CopyFromOwner(&exponent, m.numerals.Manager.Z, &fields[1].Raw.Num); err != nil {
				return err
			}
			return im.CopyFromOwner(&significand, m.numerals.Manager.Z, &fields[2].Raw.Num)
		}()
		if result != nil {
			return out, result
		}
	} else {
		result = diagnostic.BorrowProcessDebugContext().UnreachableSource("src/smt/theory_fpa.cpp", 185)
		if result == nil {
			result = ErrASTDomain
		}
		return out, result
	}
	var unbiased unumeral.Int
	defer func() { result = errors.Join(result, im.Del(&unbiased)) }()
	if result = im.Sub(&exponent, &bias, &unbiased); result != nil {
		return out, result
	}
	if result = fpaModelAssert(188, "mpzm.is_int64(exp_u)", func() (bool, error) { return im.IsInt64(&unbiased) }); result != nil {
		return out, result
	}
	frame, err := p.beginNumeralProbe()
	if err != nil {
		return out, err
	}
	var node astNodeRef
	defer func() {
		p.finishNumeralConstruction(frame, &result, &node)
		if result == nil {
			out = m.external(node)
		} else {
			out = ASTHandle{}
		}
	}()
	negative, err := im.IsOne(&sign)
	if err != nil {
		return out, err
	}
	exp, err := im.Int64(&unbiased)
	if err != nil {
		return out, err
	}
	if result = p.numeric.SetComponents(&frame.value, ebits, sbits, negative, exp, &significand); result != nil {
		return out, result
	}
	node, result = p.mkNumeral(&frame.value, nil)
	return out, result
}

func (m *ASTManager) MkFPARoundingModeModelValue(values []ASTHandle) (out ASTHandle, result error) {
	if err := m.lock(); err != nil {
		return out, err
	}
	defer m.mu.Unlock()
	defer func() {
		if result != nil {
			out = ASTHandle{}
		}
	}()
	if result = fpaModelAssert(204, "values.size() == 1", func() (bool, error) { return len(values) == 1, nil }); result != nil {
		return out, result
	}
	if len(values) < 1 {
		return out, ErrASTDomain
	}
	ref, err := m.requireExpr(values[0])
	if err != nil {
		return out, err
	}
	number, err := m.numerals.NewValue()
	if err != nil {
		return out, err
	}
	defer func() { result = errors.Join(result, m.numerals.Del(&number)) }()
	var width uint32
	ok, err := m.readBitVectorNumeral(ref, &number, &width)
	if err != nil {
		return out, err
	}
	if !ok {
		if result = diagnostic.BorrowProcessDebugContext().VerifyFailure("src/smt/theory_fpa.cpp", 215, "m_bu.is_numeral(values[0], val, bv_sz)"); result != nil {
			return out, result
		}
	}
	if result = fpaModelAssert(216, "bv_sz == 3", func() (bool, error) {
		if !ok {
			return false, ErrASTDomain
		}
		return width == 3, nil
	}); result != nil {
		return out, result
	}
	code, err := m.numerals.Uint64(&number)
	if err != nil {
		return out, err
	}
	kind := astFPARMTowardZero
	switch code {
	case 0:
		kind = astFPARMNearestEven
	case 1:
		kind = astFPARMNearestAway
	case 2:
		kind = astFPARMTowardPositive
	case 3:
		kind = astFPARMTowardNegative
	}
	p := m.floatingPointPlugin
	if p == nil || p.sourceEnded {
		return out, ErrASTHandle
	}
	node, err := m.mkFamilyApp(p.family, kind, nil, nil)
	return m.external(node), err
}
