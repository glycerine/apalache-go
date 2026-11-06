package qe

// Source: src/muz/spacer/spacer_util.cpp mbqi_project_var TRACE decisions.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *LegacyProjection) traceModelTermVerbose(line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("mbqi_project_verbose", "mbqi_project_var", "src/muz/spacer/spacer_util.cpp", line,
		func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *LegacyProjection) traceModelTermInput(variable, formula ast.ASTHandle) error {
	return p.traceModelTermVerbose(1220, func(out io.Writer) error {
		_, _ = io.WriteString(out, "MBQI: var: ")
		if err := writeExpression(p.manager, p.config, out, variable); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\nfml: ")
		if err := writeExpression(p.manager, p.config, out, formula); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}

func (p *LegacyProjection) traceModelTermCandidates(terms []ast.ASTHandle) error {
	return p.traceModelTermVerbose(1228, func(out io.Writer) error {
		_, _ = io.WriteString(out, "terms:\n")
		for i, term := range terms {
			_, _ = io.WriteString(out, strconv.Itoa(i)+": ")
			if err := writeExpression(p.manager, p.config, out, term); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
}

func (p *LegacyProjection) traceModelTermValues(term, evaluated, value ast.ASTHandle) error {
	return p.traceModelTermVerbose(1239, func(out io.Writer) error {
		_, _ = io.WriteString(out, "term: ")
		if err := writeExpression(p.manager, p.config, out, term); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " tval: ")
		if err := writeExpression(p.manager, p.config, out, evaluated); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " val: ")
		if err := writeExpression(p.manager, p.config, out, value); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}

func (p *LegacyProjection) traceModelTermProjection(variable, term ast.ASTHandle, replaced bool) error {
	tags := p.config.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	line := 1256
	if replaced {
		line = 1247
	}
	err := tags.EmitConditional("mbqi_project", "mbqi_project_var", "src/muz/spacer/spacer_util.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			writer := traceBodyWriter{out}
			prefix, separator := "MBQI: failed to eliminate ", " from "
			if replaced {
				prefix, separator = "MBQI: replacing ", " with "
			}
			_, _ = io.WriteString(writer, prefix)
			if err := writeExpression(p.manager, p.config, writer, variable); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, separator)
			if err := writeExpression(p.manager, p.config, writer, term); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// Source: model_evaluator_util::eval's typed exception diagnostic.
func (p *LegacyProjection) traceModelTermEvaluationFailure(failure error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer_model_evaluator", "eval", "src/muz/spacer/spacer_util.cpp", 101,
		func() bool { return true }, func(out io.Writer) error { _, err := io.WriteString(out, failure.Error()+"\n"); return err })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
