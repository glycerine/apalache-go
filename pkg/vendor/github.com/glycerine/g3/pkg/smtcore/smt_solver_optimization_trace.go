package smtcore

// Source: src/opt/opt_solver.cpp:162-167.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *SMTSolver) writeOptimizationExpression(out io.Writer, expression ast.ASTHandle) (resultErr error) {
	empty := s.config.Params.EmptyValue()
	document, err := s.manager.SMT2PPExprTreeWithParams(expression, nil, s.config.Params, s.kernel.paramGlobals, &empty)
	if err != nil {
		return err
	}
	formats, err := s.manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(document)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	if err = s.manager.PrintFormat(stream, document, s.config.Params, &empty, s.kernel.paramGlobals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func (s *SMTSolver) traceOptimizationCheck() error {
	if s.optimization == nil {
		return nil
	}
	tags := s.config.ModelEvaluator.Trace
	if tags == nil {
		tags = s.manager.BorrowSourceTraceTags()
	}
	if !tags.ShouldTrace("opt_verbose") {
		return nil
	}
	var operandErr error
	err := tags.EmitConditional("opt_verbose", "check_sat_core", "src/opt/opt_solver.cpp", 162, func() bool { return true }, func(writer io.Writer) error {
		out := &optimizationBenchmarkWriter{out: writer}
		_, _ = io.WriteString(out, "context size: "+strconv.Itoa(len(s.kernel.assertions))+"\n")
		for i := 0; i < len(s.kernel.assertions); i++ {
			if operandErr = s.writeOptimizationExpression(out, s.kernel.assertions[i].Formula); operandErr != nil {
				return operandErr
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
