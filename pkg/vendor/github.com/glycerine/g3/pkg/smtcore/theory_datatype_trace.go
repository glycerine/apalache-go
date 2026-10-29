package smtcore

// Source: src/smt/theory_datatype.cpp retained internalization/equality TRACE.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *SMTDatatypeTheory) displayDatatypeBounded(out io.Writer, expression ast.ASTHandle) (result error) {
	s, err := t.State()
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = s.manager.DisplayBoundedLowLevel(stream, expression, 3); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}

func (t *SMTDatatypeTheory) traceDatatypeOccursNode(function string, line int, heading string, node *SMTEnode, suffix string) error {
	return t.emitDatatype("datatype", function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, heading)
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "%d ", id)
		if err = t.displayDatatypeBounded(out, node.owner); err != nil {
			return err
		}
		_, err = io.WriteString(out, suffix+"\n")
		return err
	})
}

func (c *smtDatatypeOccursCheck) traceConflict() error {
	return c.theory.emitDatatype("datatype", "occurs_check", 556, func(out io.Writer) error {
		_, _ = io.WriteString(out, "occurs_check: true\n")
		for _, pair := range c.equalities {
			_, _ = io.WriteString(out, "eq: #")
			first, err := pair.First.OwnerID()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "%d #", first)
			second, err := pair.Second.OwnerID()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "%d\n", second)
			if err = c.theory.displayDatatypeBounded(out, pair.First.owner); err != nil {
				return err
			}
			_, _ = io.WriteString(out, " ")
			if err = c.theory.displayDatatypeBounded(out, pair.Second.owner); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
}

func (t *SMTDatatypeTheory) emitDatatype(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/theory_datatype.cpp", line, func() bool { return true }, func(out io.Writer) error {
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

func (t *SMTDatatypeTheory) traceDatatypeRecognizerExhaustion(trigger *SMTEnode, literals []SMTLiteral, equalities []SMTEnodePair) error {
	return t.emitDatatype("datatype_conflict", "propagate_recognizer", 780, func(out io.Writer) error {
		s, err := t.State()
		if err != nil {
			return err
		}
		if trigger == nil {
			return ast.ErrASTDomain
		}
		if err = t.displayDatatypeExpression(out, trigger.owner); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		for _, literal := range literals {
			if err = literal.Display(out, s.manager, s.boolExpr); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
		}
		for _, pair := range equalities {
			if err = t.displayDatatypeExpression(out, pair.First.owner); err != nil {
				return err
			}
			_, _ = io.WriteString(out, " = ")
			if err = t.displayDatatypeExpression(out, pair.Second.owner); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
}

func (t *SMTDatatypeTheory) traceDatatypeEmptyRecognizers() error {
	return t.emitDatatype("datatype", "propagate_recognizer", 750, func(out io.Writer) error {
		s, err := t.State()
		if err != nil {
			return err
		}
		return s.DisplayContext(out)
	})
}

func (t *SMTDatatypeTheory) displayDatatypeExpression(out io.Writer, expression ast.ASTHandle) (result error) {
	s, err := t.State()
	if err != nil {
		return err
	}
	format, err := s.manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = s.manager.PrintFormat(stream, format, nil, nil, nil); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}

func (t *SMTDatatypeTheory) traceDatatypeExpression(function string, line int, heading string, expression ast.ASTHandle) error {
	return t.traceDatatypeTaggedExpression("datatype", function, line, heading, expression)
}

func (t *SMTDatatypeTheory) traceDatatypeTaggedExpression(tag, function string, line int, heading string, expression ast.ASTHandle) error {
	return t.emitDatatype(tag, function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, heading)
		if err := t.displayDatatypeExpression(out, expression); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
