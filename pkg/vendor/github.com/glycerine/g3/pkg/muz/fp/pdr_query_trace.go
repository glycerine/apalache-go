package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp:68,144 reset/query traces.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (e *PropositionalEngine) tracePDRFreshRule(index int) error {
	tags := e.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "check_reset", "src/muz/pdr/pdr_dl_interface.cpp", 68,
		func() bool { return true }, func(out io.Writer) (resultErr error) {
			rule, err := e.set.Rule(index)
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
			writer := pdrTraceOperandWriter{out: out}
			_, _ = io.WriteString(writer, "Fresh rule ")
			return e.writeRuleDisplay(writer, rule)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// The trace owner records ostream failures. Suppress their Go write returns
// here so subsequent source operands (including dump exceptions) still run.
type pdrTraceOperandWriter struct{ out io.Writer }

func (w pdrTraceOperandWriter) Write(body []byte) (int, error) {
	_, _ = w.out.Write(body)
	return len(body), nil
}

func (e *PropositionalEngine) tracePDRTransformedRules(rules *base.RuleSet) error {
	tags := e.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	outputs, err := rules.OutputPredicates()
	if err != nil || len(outputs) == 0 {
		return err
	}
	err = tags.EmitConditional("pdr", "query", "src/muz/pdr/pdr_dl_interface.cpp", 144,
		func() bool { return true }, func(out io.Writer) (resultErr error) {
			writer := pdrTraceOperandWriter{out: out}
			_, _ = io.WriteString(writer, "rules:\n")
			if err := e.writeRuleSetDisplay(writer, rules); err != nil {
				return err
			}
			config, err := e.contextSMT2Config(nil, nil)
			if err != nil {
				return err
			}
			return rules.WriteContextSMT2(writer, config)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (e *PropositionalEngine) tracePDRInput(declaration ast.ASTHandle) error {
	tags := e.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "query", "src/muz/pdr/pdr_dl_interface.cpp", 92,
		func() bool { return true }, func(out io.Writer) (resultErr error) {
			writer := pdrTraceOperandWriter{out: out}
			var roots []ast.ASTRef
			defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, roots)) }()
			own := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
				if err != nil {
					return h, err
				}
				r, err := e.manager.NewRef(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				roots = append(roots, r)
				return h, nil
			}
			background, err := e.manager.RefGet(e.background)
			if err != nil {
				return err
			}
			if len(e.assertions) > 0 {
				var axioms []ast.ASTHandle
				for _, ref := range e.assertions {
					h, err := e.manager.RefGet(ref)
					if err != nil {
						return err
					}
					axioms = append(axioms, h)
				}
				if len(axioms) == 1 {
					background = axioms[0]
				} else {
					background, err = own(e.manager.MkAndArray(axioms))
					if err != nil {
						return err
					}
				}
			}
			if !e.manager.IsTrue(background) {
				_, _ = io.WriteString(writer, "axioms:\n")
				if err = e.writePDRTraceExpression(writer, background); err != nil {
					return err
				}
				_, _ = io.WriteString(writer, "\n")
			}
			query := e.pdrTraceQuery
			if query.IsNull() {
				count, err := e.manager.GetArity(declaration)
				if err != nil {
					return err
				}
				args := make([]ast.ASTHandle, count)
				for i := uint32(0); i < count; i++ {
					sort, err := e.manager.GetDomain(declaration, i)
					if err != nil {
						return err
					}
					args[i], err = own(e.manager.MkVar(i, sort))
					if err != nil {
						return err
					}
				}
				query, err = own(e.manager.MkApp(declaration, args))
				if err != nil {
					return err
				}
			}
			_, _ = io.WriteString(writer, "query: ")
			if err = e.writePDRTraceExpression(writer, query); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, "\nrules:\n")
			shadow, err := snapshotHornRules(e.set)
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, shadow.Close()) }()
			if err = shadow.SetOutputPredicate(declaration); err != nil {
				return err
			}
			return e.writeRuleSetDisplay(writer, shadow)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (e *PropositionalEngine) writePDRTraceExpression(out io.Writer, h ast.ASTHandle) (resultErr error) {
	format, err := e.manager.SMT2PPExprTreeWithParams(h, nil, e.params, e.globals, nil)
	if err != nil {
		return err
	}
	formats, err := e.manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: e.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	empty := e.params.EmptyValue()
	if err = e.manager.PrintFormat(stream, format, e.params, &empty, e.globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
