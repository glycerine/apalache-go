package cmd_context

// Source: src/cmd_context/echo_tactic.h; src/cmd_context/echo_tactic.cpp.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type echoTactic struct {
	self       *echoTactic
	context    *Context
	probe      tactic.ProbeEvaluator
	message    *string
	references uint32
	newline    bool
	closed     bool
}

var _ tactic.Tactic = (*echoTactic)(nil)

// NewEchoTactic constructs the regular-stream echo tactic. The context and
// its output transport remain borrowed for the tactic's lifetime.
func NewEchoTactic(context *Context, message string, newline bool) (tactic.Tactic, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	if err := context.valid(); err != nil {
		return nil, err
	}
	t := &echoTactic{context: context, message: &message, newline: newline}
	t.self = t
	return t, nil
}

// NewProbeValueTactic constructs the diagnostic-stream probe-value tactic.
// A nil message preserves the source's no-prefix branch. The tactic retains
// one reference to probe until its final DecRef.
func NewProbeValueTactic(context *Context, message *string, probe tactic.ProbeEvaluator, newline bool) (result tactic.Tactic, err error) {
	if context == nil || probe == nil {
		return nil, ast.ErrASTHandle
	}
	if err = context.valid(); err != nil {
		return nil, err
	}
	if err = probe.Check(); err != nil {
		return nil, err
	}
	if err = probe.IncRef(); err != nil {
		return nil, err
	}
	t := &echoTactic{context: context, probe: probe, message: message, newline: newline}
	t.self = t
	return t, nil
}

func (t *echoTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	if t.context == nil {
		return ast.ErrASTHandle
	}
	return t.context.valid()
}

func (t *echoTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}

func (t *echoTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references != 0 {
		return nil
	}
	t.closed = true
	if t.probe != nil {
		probe := t.probe
		t.probe = nil
		return probe.DecRef()
	}
	return nil
}

func echoWrite(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	return err
}

func (t *echoTactic) Apply(in *tactic.Goal, result *tactic.TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if in == nil || result == nil {
		return ast.ErrASTHandle
	}
	if err := in.CheckOwner(); err != nil {
		return err
	}
	if t.probe == nil {
		if t.message == nil {
			return ast.ErrASTHandle
		}
		if err := echoWrite(contextRegularOutput{t.context}, *t.message); err != nil {
			return err
		}
		if t.newline {
			if err := echoWrite(contextRegularOutput{t.context}, "\n"); err != nil {
				return err
			}
		}
	} else {
		value, err := t.probe.Eval(in)
		if err != nil {
			return err
		}
		out := contextDiagnosticOutput{t.context}
		if t.message != nil {
			if err = echoWrite(out, *t.message+" "); err != nil {
				return err
			}
		}
		if err = echoWrite(out, strconv.FormatFloat(value, 'g', 6, 64)); err != nil {
			return err
		}
		if t.newline {
			if err = echoWrite(out, "\n"); err != nil {
				return err
			}
		}
	}
	if err := result.Prepare(in.Manager()); err != nil {
		return err
	}
	return result.AddGoal(in)
}

func (t *echoTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *echoTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *echoTactic) CollectStatistics() map[string]float64         { return nil }
func (t *echoTactic) ResetStatistics() error                        { return t.open() }
func (t *echoTactic) Cleanup() error                                { return t.open() }
func (t *echoTactic) Reset() error                                  { return t.open() }
func (t *echoTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *echoTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *echoTactic) Translate(*ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t, nil
}
