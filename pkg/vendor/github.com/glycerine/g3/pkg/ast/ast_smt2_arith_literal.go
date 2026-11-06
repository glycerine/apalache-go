package ast

// Source: src/ast/ast_smt2_pp.cpp:296-368.

import (
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type smt2AlgebraicText struct{ strings.Builder }

func (s *smt2AlgebraicText) AppendInt32(value int32) error {
	_, err := s.WriteString(strconv.FormatInt(int64(value), 10))
	return err
}
func (s *smt2AlgebraicText) AppendCString(value string) error {
	_, err := s.WriteString(value)
	return err
}
func (s *smt2AlgebraicText) AppendNumeralText(value string) error {
	_, err := s.WriteString(value)
	return err
}

// SMT2PPArithLiteral formats the concrete arithmetic numeral branches used by
// the SMT-LIB printer. matched is false for non-arithmetic constants.
func (m *ASTManager) SMT2PPArithLiteral(h ASTHandle, decimal bool, precision uint32) (format ASTHandle, matched bool, err error) {
	return m.smt2PPArithLiteral(h, decimal, precision)
}

func (m *ASTManager) smt2PPArithLiteral(h ASTHandle, decimal bool, precision uint32) (format ASTHandle, matched bool, result error) {
	value, integer, matched, err := m.ArithmeticNumeralText(h)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if !matched {
		family, e := m.GetFamilyID(h)
		if e != nil {
			return ASTHandle{}, false, e
		}
		arithmetic, e := m.ArithmeticFamilyID()
		if e != nil || family != arithmetic {
			return ASTHandle{}, false, e
		}
		var algebraic polynomial.Algebraic
		matched, e = m.ArithmeticAlgebraicNumeralValue(h, &algebraic)
		if e != nil || !matched {
			return ASTHandle{}, matched, e
		}
		manager, e := m.ArithmeticAlgebraicManager()
		if e != nil {
			return ASTHandle{}, true, e
		}
		defer func() { result = errors.Join(result, manager.Del(&algebraic)) }()
		var text smt2AlgebraicText
		negative := false
		if decimal {
			negative, e = manager.IsNeg(&algebraic)
			if e == nil && negative {
				e = manager.Neg(&algebraic)
			}
			if e == nil {
				e = manager.DisplayDecimal(&text, &algebraic, precision)
			}
		} else {
			e = manager.DisplayRootSMT2Into(&text, &algebraic)
		}
		if e != nil {
			return ASTHandle{}, true, e
		}
		format, e = m.FormatString(text.String())
		if e == nil && decimal && negative {
			format, e = m.FormatSeq1("-", []ASTHandle{format})
		}
		return format, true, e
	}
	var rational big.Rat
	if _, ok := rational.SetString(value); !ok {
		return ASTHandle{}, true, ErrASTDomain
	}
	negative := rational.Sign() < 0
	if negative {
		rational.Neg(&rational)
	}
	if integer {
		if !rational.IsInt() {
			return ASTHandle{}, true, ErrASTDomain
		}
		format, err = m.FormatString(rational.Num().String())
	} else if rational.IsInt() {
		format, err = m.FormatString(rational.Num().String() + ".0")
	} else if decimal {
		service := m.NumeralService()
		if service == nil {
			return ASTHandle{}, true, ErrASTHandle
		}
		var owned unumeral.OwnedRational
		owned, err = service.NewValue()
		if err != nil {
			return ASTHandle{}, true, err
		}
		defer func() { result = errors.Join(result, service.Del(&owned)) }()
		if err = service.SetString(&owned, rational.RatString()); err == nil {
			var text string
			text, err = service.DecimalString(&owned, precision, false)
			if err == nil {
				format, err = m.FormatString(text)
			}
		}
	} else {
		numerator, e := m.FormatString(rational.Num().String() + ".0")
		if e != nil {
			return ASTHandle{}, true, e
		}
		denominator, e := m.FormatString(rational.Denom().String() + ".0")
		if e != nil {
			return ASTHandle{}, true, e
		}
		format, err = m.FormatSeq1("/", []ASTHandle{numerator, denominator})
	}
	if err != nil {
		return ASTHandle{}, true, err
	}
	if negative {
		format, err = m.FormatSeq1("-", []ASTHandle{format})
	}
	return format, true, err
}
