package smtcore

// Source: src/smt/theory_fpa.cpp:129-132,194-198,206-209,228.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *smtFPAValueProcedure) modelTraceTags(generator *SMTModelGenerator) *diagnostic.TraceTags {
	if generator != nil && generator.trace != nil {
		return generator.trace
	}
	return p.manager.BorrowSourceTraceTags()
}

func (p *smtFPAValueProcedure) emitModelTrace(generator *SMTModelGenerator, tag string, line int, body func(io.Writer) error) error {
	tags := p.modelTraceTags(generator)
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var operandErr error
	err := tags.EmitConditional(tag, "mk_value", "src/smt/theory_fpa.cpp", line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *smtFPAValueProcedure) displayModelTraceExpression(out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

func (p *smtFPAValueProcedure) traceInputs(generator *SMTModelGenerator, values []ast.ASTHandle) error {
	line := 129
	if p.rounding {
		line = 206
	}
	return p.emitModelTrace(generator, "t_fpa_detail", line, func(out io.Writer) error {
		for i, value := range values {
			if _, err := fmt.Fprintf(out, "value[%d] = ", i); err != nil {
				return err
			}
			if err := p.displayModelTraceExpression(out, value); err != nil {
				return err
			}
			if err := diagnostic.WriteTraceEndl(out); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *smtFPAValueProcedure) readModelBitVector(value ast.ASTHandle) (text string, width uint32, result error) {
	service := p.manager.NumeralService()
	number, err := service.NewValue()
	if err != nil {
		return "", 0, err
	}
	defer func() { result = errors.Join(result, service.Del(&number)) }()
	found, err := p.manager.BitVectorNumeralValue(value, &number, &width)
	if err != nil {
		return "", 0, err
	}
	if !found {
		return "", 0, ast.ErrASTDomain
	}
	text, result = service.String(&number)
	return text, width, result
}

func (p *smtFPAValueProcedure) modelComponents(values []ast.ASTHandle) ([3]string, error) {
	var components [3]string
	if len(values) == 3 {
		for i, value := range values {
			text, _, err := p.readModelBitVector(value)
			if err != nil {
				return components, err
			}
			components[i] = text
		}
		return components, nil
	}
	if len(values) != 1 || p.ebits < 2 || p.sbits < 2 {
		return components, ast.ErrASTDomain
	}
	text, width, err := p.readModelBitVector(values[0])
	if err != nil || width != p.ebits+p.sbits {
		if err != nil {
			return components, err
		}
		return components, ast.ErrASTDomain
	}
	packed, ok := new(big.Int).SetString(text, 10)
	if !ok || packed.Sign() < 0 {
		return components, ast.ErrASTDomain
	}
	components[0] = new(big.Int).Rsh(new(big.Int).Set(packed), uint(width-1)).String()
	exponent := new(big.Int).Rsh(new(big.Int).Set(packed), uint(p.sbits-1))
	exponent.And(exponent, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(p.ebits)), big.NewInt(1)))
	components[1] = exponent.String()
	significand := new(big.Int).And(new(big.Int).Set(packed), new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(p.sbits-1)), big.NewInt(1)))
	components[2] = significand.String()
	return components, nil
}

func (p *smtFPAValueProcedure) traceResult(generator *SMTModelGenerator, values []ast.ASTHandle, value ast.ASTHandle) error {
	line := 194
	if p.rounding {
		line = 228
	}
	return p.emitModelTrace(generator, "t_fpa", line, func(out io.Writer) error {
		if p.rounding {
			if _, err := io.WriteString(out, "result: "); err != nil {
				return err
			}
		} else {
			components, err := p.modelComponents(values)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "result: [%s,%s,%s] --> ", components[0], components[1], components[2]); err != nil {
				return err
			}
		}
		if err := p.displayModelTraceExpression(out, value); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}
