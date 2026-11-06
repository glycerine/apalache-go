package smtcore

// Source: TRACE sites in src/smt/tactic/smt_tactic.cpp:53,78,128,
// 157-165,175-176,231,298.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// smtTacticTraceOutput gives trace operands ostream badbit semantics. The
// TraceTags owner observes and retains the first refusal; later operands still
// execute, as they do after native tout enters the failed state.
type smtTacticTraceOutput struct{ io.Writer }

func (w smtTacticTraceOutput) Write(p []byte) (int, error) {
	_, _ = w.Writer.Write(p)
	return len(p), nil
}

func (t *SMTTactic) emitTrace(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/tactic/smt_tactic.cpp", line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(smtTacticTraceOutput{out})
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

func smtTacticBool(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (t *SMTTactic) withTraceGlobals(body func() error) (result error) {
	if uparams.BorrowProcessGlobalParamsService() == t.config.Globals {
		return body()
	}
	previous, err := uparams.BindProcessGlobalParamsService(t.config.Globals)
	if err != nil {
		return err
	}
	defer func() {
		_, restore := uparams.BindProcessGlobalParamsService(previous)
		result = errors.Join(result, restore)
	}()
	return body()
}

func (t *SMTTactic) traceParams(prefix, function string, line int, p *uparams.ParamsRef) error {
	return t.emitTrace("smt_tactic", function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, prefix)
		if err := t.config.Params.DisplayTo(out, p); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}

func (t *SMTTactic) traceApplyState(in *tactic.Goal) error {
	if err := t.emitTrace("smt_tactic", "operator()", 157, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "%p\nAUTO_CONFIG: %s HIDIV0: %s  PREPROCESS: %s\n", t,
			smtTacticBool(t.fullParams.state.AutoConfig), smtTacticBool(t.fullParams.bv.state.HiDiv0), smtTacticBool(t.fullParams.state.Preprocess))
		_, _ = fmt.Fprintf(out, "RELEVANCY: %d\nfail-if-inconclusive: %s\nparams_ref: ", t.fullParams.state.RelevancyLvl, smtTacticBool(t.failIfInconclusive))
		if err := t.config.Params.DisplayTo(out, &t.params); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\nnnf: "+smtTacticBool(t.fullParams.preprocess.state.NNFCNF)+"\n")
		return nil
	}); err != nil {
		return err
	}
	if err := t.emitTrace("smt_tactic_params", "operator()", 163, func(out io.Writer) (result error) {
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, stream.Close()) }()
		// Native reads the otherwise uninitialized m_qi_ematching bit while
		// tracing. Use a deterministic zero only for this display operation;
		// the solver's parameter state remains untouched.
		ematchingInitialized := t.fullParams.qi.state.EmatchingInitialized
		if !ematchingInitialized {
			t.fullParams.qi.state.EmatchingInitialized = true
			defer func() { t.fullParams.qi.state.EmatchingInitialized = false }()
		}
		if err = t.fullParams.Display(stream); err != nil {
			return err
		}
		text, err := stream.String()
		if err != nil {
			return err
		}
		_, _ = io.WriteString(out, text)
		return nil
	}); err != nil {
		return err
	}
	if err := t.emitTrace("smt_tactic_detail", "operator()", 164, func(out io.Writer) error {
		return t.withTraceGlobals(func() error { return in.Display(out) })
	}); err != nil {
		return err
	}
	return t.emitTrace("smt_tactic_memory", "operator()", 165, func(out io.Writer) error {
		// Go's budget owner tracks charged bytes, not allocator fragmentation.
		_, _ = io.WriteString(out, "wasted_size: 0\n")
		return nil
	})
}

func (t *SMTTactic) traceLogic() error {
	return t.emitTrace("smt_tactic", "scoped_init_ctx", 128, func(out io.Writer) error {
		text := "null"
		var err error
		if t.logic != (usymbol.Symbol{}) {
			text, err = t.manager.Symbols.String(t.logic)
			if err != nil {
				return err
			}
		}
		_, _ = io.WriteString(out, "logic: "+text+"\n")
		return nil
	})
}

func (t *SMTTactic) traceMUS(in *tactic.Goal, extraction *solver.ClauseExtraction) error {
	return t.emitTrace("mus", "operator()", 175, func(out io.Writer) error {
		if err := t.withTraceGlobals(func() error { return in.DisplayWithDependencies(out) }); err != nil {
			return err
		}
		printer, err := ast.NewSimpleASTPrinterContext(t.manager, out, out)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(out, "(")
		for index, clause := range extraction.Clauses {
			if index != 0 {
				_, _ = io.WriteString(out, " ")
			}
			if err = printer.DisplayExpr(out, clause, 0); err != nil {
				return err
			}
		}
		_, _ = io.WriteString(out, ")\n")
		return nil
	})
}

func (t *SMTTactic) traceApproximationFailure() error {
	return t.emitTrace("smt_tactic", "operator()", 231, func(out io.Writer) error {
		_, _ = io.WriteString(out, "failed to show to be unsat...\n")
		return nil
	})
}

func (t *SMTTactic) traceUsingFactory(autoConfig bool, p *uparams.ParamsRef) error {
	return t.emitTrace("smt_tactic", "mk_smt_tactic_using", 298, func(out io.Writer) error {
		_, _ = io.WriteString(out, "auto_config: "+smtTacticBool(autoConfig)+"\nr: "+fmt.Sprintf("%p", t)+"\np: ")
		if err := t.config.Params.DisplayTo(out, p); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}
