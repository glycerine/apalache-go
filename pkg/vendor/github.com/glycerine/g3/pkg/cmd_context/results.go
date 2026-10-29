package cmd_context

// Source: src/cmd_context/eval_cmd.cpp; src/cmd_context/basic_cmds.cpp
// get-value, get-assignment,
// get-consequences, get-proof and get-unsat-core; cmd_context.cpp result
// availability checks.
// Original get-value term text follows smt2parser.cpp parse_get_value.

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// EvaluationOptions are the command-local parameters accepted by eval_cmd.
// Nil fields preserve the corresponding context/global evaluator parameter.
type EvaluationOptions struct {
	MaxMemory, MaxSteps                *uint32
	Completion, Cache, ArrayEqualities *bool
	Timeout, RLimit, ModelIndex        uint32
}

type contextTimeoutHandler struct {
	mu            sync.Mutex
	limit         *control.Reslimit
	active, fired bool
}

func (h *contextTimeoutHandler) HandleEvent(control.EventCaller) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active || h.fired {
		return
	}
	h.limit.IncCancel()
	h.fired = true
}

func (h *contextTimeoutHandler) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
	if h.fired {
		h.limit.DecCancel()
	}
}

func (c *Context) WriteExpression(out io.Writer, expression ast.ASTHandle) (result error) {
	format, err := c.manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := c.config.Params.EmptyValue()
	if err = c.manager.PrintFormat(stream, format, c.config.Params, &empty, c.config.Globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func (c *Context) WriteValues(out io.Writer, expressions []ast.ASTHandle) (result error) {
	return c.WriteValuesWithText(out, expressions, nil)
}

// Evaluate evaluates an expression in the latest model and returns one owning
// AST reference. The caller must release it through Manager.DelRef.
func (c *Context) Evaluate(expression ast.ASTHandle, completion bool) (result ast.ASTRef, resultErr error) {
	return c.EvaluateWithOptions(expression, EvaluationOptions{Completion: &completion})
}

// EvaluateWithOptions evaluates an expression using command-local evaluator
// parameters. The caller owns the returned AST reference.
func (c *Context) EvaluateWithOptions(expression ast.ASTHandle, options EvaluationOptions) (result ast.ASTRef, resultErr error) {
	md, err := c.Model()
	if err != nil {
		return result, err
	}
	if options.ModelIndex != 0 && len(c.objectives) != 0 {
		if err = md.DecRef(); err != nil {
			return result, err
		}
		if uint64(options.ModelIndex) >= uint64(len(c.optimizationBoxModels)) {
			return result, fmt.Errorf("index into models is out of bounds")
		}
		index := int(options.ModelIndex)
		md = c.optimizationBoxModels[index]
		if md == nil {
			return result, fmt.Errorf("model is not available")
		}
		if err = md.IncRef(); err != nil {
			return result, err
		}
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	params, err := c.config.Params.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&params)) }()
	if err = c.config.Params.Append(&params, &c.params); err != nil {
		return result, err
	}
	setUInt := func(name string, value *uint32) error {
		if value == nil {
			return nil
		}
		return c.config.Params.SetUIntName(&params, &name, *value)
	}
	setBool := func(name string, value *bool) error {
		if value == nil {
			return nil
		}
		return c.config.Params.SetBoolName(&params, &name, *value)
	}
	if err = setUInt("max_memory", options.MaxMemory); err != nil {
		return result, err
	}
	if err = setUInt("max_steps", options.MaxSteps); err != nil {
		return result, err
	}
	if err = setBool("completion", options.Completion); err != nil {
		return result, err
	}
	if err = setBool("cache", options.Cache); err != nil {
		return result, err
	}
	if err = setBool("array_equalities", options.ArrayEqualities); err != nil {
		return result, err
	}
	config := c.config.ModelEvaluator
	config.Params = &params
	evaluator, err := model.NewModelEvaluator(md, config)
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	}
	if err != nil {
		return result, err
	}
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	if err = manager.ResourceLimit().Push(options.RLimit); err != nil {
		return result, err
	}
	defer manager.ResourceLimit().Pop()
	handler := &contextTimeoutHandler{limit: manager.ResourceLimit(), active: true}
	timer, err := control.NewScopedTimer(options.Timeout, handler)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, timer.Close())
		handler.close()
	}()
	return evaluator.EvaluateValue(expression)
}

// WriteEvaluation evaluates one expression and prints its value using the
// command context's SMT-LIB expression printer.
func (c *Context) WriteEvaluation(out io.Writer, expression ast.ASTHandle, completion bool) (resultErr error) {
	return c.WriteEvaluationWithOptions(out, expression, EvaluationOptions{Completion: &completion})
}

// WriteEvaluationWithOptions implements eval_cmd's parameterized evaluation
// and its recoverable model-evaluator diagnostic.
func (c *Context) WriteEvaluationWithOptions(out io.Writer, expression ast.ASTHandle, options EvaluationOptions) (resultErr error) {
	value, err := c.EvaluateWithOptions(expression, options)
	if err != nil {
		if rewriter.IsRewriterException(err) {
			message := strings.ReplaceAll(err.Error(), "\"", "\"\"")
			_, resultErr = fmt.Fprintf(out, "(error \"evaluator failed: %s\")\n", message)
			return resultErr
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&value)) }()
	handle, err := c.manager.RefGet(value)
	if err != nil {
		return err
	}
	if err = c.WriteExpression(out, handle); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

func (c *Context) WriteAssertions(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if !c.InteractiveMode {
		return fmt.Errorf("command is only available in interactive mode, use command (set-option :interactive-mode true)")
	}
	if _, err := io.WriteString(out, "("); err != nil {
		return err
	}
	for i := range c.assertions {
		if i > 0 {
			if _, err := io.WriteString(out, "\n "); err != nil {
				return err
			}
		}
		if c.assertions[i].displayText != "" {
			if _, err := io.WriteString(out, c.assertions[i].displayText); err != nil {
				return err
			}
		} else if err := c.WriteExpression(out, c.assertions[i].expression); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func (c *Context) WriteAssignment(out io.Writer) (result error) {
	md, err := c.Model()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, md.DecRef()) }()
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if evaluator != nil {
		defer func() { result = errors.Join(result, evaluator.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	boolSort, err := c.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "("); err != nil {
		return err
	}
	first := true
	seen := make(map[string]bool)
	for _, name := range c.macroTrail {
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, declaration := range c.macros[name] {
			if len(declaration.domain) != 0 {
				continue
			}
			sort, e := c.manager.GetSort(declaration.body)
			if e != nil {
				return e
			}
			if sort != boolSort {
				continue
			}
			value, e := evaluator.EvaluateValue(declaration.body)
			if e != nil {
				return e
			}
			h, e := c.manager.RefGet(value)
			if e == nil && (c.manager.IsTrue(h) || c.manager.IsFalse(h)) {
				e = c.writeAssignmentPair(out, name, c.manager.IsTrue(h), &first)
			}
			e = errors.Join(e, c.manager.DelRef(&value))
			if e != nil {
				return e
			}
		}
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

func (c *Context) WriteConsequences(out io.Writer, assumptions, variables []ast.ASTHandle) error {
	status, consequences, err := c.Consequences(assumptions, variables)
	if err != nil {
		return err
	}
	word := "unknown"
	if status == lbool.True {
		word = "sat"
	} else if status == lbool.False {
		word = "unsat"
	}
	if _, err = fmt.Fprintln(out, word); err != nil {
		return err
	}
	if _, err = fmt.Fprintln(out); err != nil {
		return err
	}
	for _, consequence := range consequences {
		if err = c.WriteExpression(out, consequence); err != nil {
			return fmt.Errorf("write consequence: %w", err)
		}
		if _, err = fmt.Fprintln(out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) writeAssignmentPair(out io.Writer, name string, value bool, first *bool) error {
	symbol, err := c.manager.Symbols.Intern(name)
	if err != nil {
		return err
	}
	name, err = c.smt2SymbolName(symbol)
	if err != nil {
		return err
	}
	if !*first {
		if _, err = io.WriteString(out, " "); err != nil {
			return err
		}
	}
	*first = false
	_, err = fmt.Fprintf(out, "(%s %t)", name, value)
	return err
}

func (c *Context) smt2SymbolName(symbol usymbol.Symbol) (string, error) {
	name, err := c.manager.Symbols.String(symbol)
	if err != nil {
		return "", err
	}
	quoted, err := usmt2.IsSMT2QuotedSymbol(c.manager.Symbols, symbol)
	if err != nil {
		return "", err
	}
	if quoted {
		name, err = usmt2.MkSMT2QuotedSymbol(c.manager.Symbols, symbol, nil)
		if err != nil {
			return "", err
		}
	}
	return name, nil
}

// WriteLabels displays the relevant Simplify-style labels from the latest
// satisfiable or unknown result.
func (c *Context) WriteLabels(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if _, err := c.Manager(); err != nil {
		return err
	}
	if !c.checked || c.status != lbool.True && c.status != lbool.Undef {
		return fmt.Errorf("labels are not available")
	}
	labels, err := c.checkSatResult().Labels()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "(labels"); err != nil {
		return err
	}
	for _, label := range labels {
		name, e := c.smt2SymbolName(label)
		if e != nil {
			return e
		}
		if _, err = fmt.Fprintf(out, " %s", name); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

// WriteValuesWithText evaluates parsed expressions while echoing their original
// SMT-LIB terms. A nil text slice uses the ordinary AST expression printer.
func (c *Context) WriteValuesWithText(out io.Writer, expressions []ast.ASTHandle, text []string) (result error) {
	if text != nil && len(text) != len(expressions) {
		return fmt.Errorf("get-value term text count does not match expression count")
	}
	md, err := c.Model()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, md.DecRef()) }()
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if evaluator != nil {
		defer func() { result = errors.Join(result, evaluator.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "("); err != nil {
		return err
	}
	for i, expression := range expressions {
		if i > 0 {
			if _, err = io.WriteString(out, "\n "); err != nil {
				return err
			}
		}
		value, err := evaluator.EvaluateValue(expression)
		if err != nil {
			return err
		}
		term := ""
		if text != nil {
			term = text[i]
		}
		err = c.writeValuePair(out, expression, value, term)
		err = errors.Join(err, c.manager.DelRef(&value))
		if err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

func (c *Context) writeValuePair(out io.Writer, expression ast.ASTHandle, value ast.ASTRef, text string) error {
	h, err := c.manager.RefGet(value)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "("); err != nil {
		return err
	}
	if text == "" {
		err = c.WriteExpression(out, expression)
	} else {
		_, err = io.WriteString(out, text)
	}
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, " "); err != nil {
		return err
	}
	if err = c.WriteExpression(out, h); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")")
	return err
}

func (c *Context) WriteUnsatCore(out io.Writer) error {
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if !c.checked || c.status != lbool.False || !state.UnsatCore {
		return fmt.Errorf("unsat core is not available")
	}
	return c.writeUnsatResultCore(out)
}

func (c *Context) WriteUnsatAssumptions(out io.Writer) error {
	if !c.ProduceUnsatAssumptions {
		return fmt.Errorf("unsat assumptions construction is not enabled, use command (set-option :produce-unsat-assumptions true)")
	}
	if !c.checked || c.status != lbool.False {
		return fmt.Errorf("unsat assumptions is not available")
	}
	return c.writeUnsatResultCore(out)
}

func (c *Context) writeUnsatResultCore(out io.Writer) error {
	core, err := c.checkSatResult().UnsatCore()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "("); err != nil {
		return err
	}
	for i, expression := range core {
		if i > 0 {
			if _, err = io.WriteString(out, " "); err != nil {
				return err
			}
		}
		if err = c.WriteExpression(out, expression); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

func (c *Context) WriteProof(out io.Writer) error {
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if !state.Proof {
		return fmt.Errorf("proof construction is not enabled, use command (set-option :produce-proofs true)")
	}
	if !c.checked || c.status != lbool.False {
		return fmt.Errorf("proof is not available")
	}
	proof, err := c.checkSatResult().Proof()
	if err != nil {
		return err
	}
	if proof.IsNull() {
		return fmt.Errorf("proof is not available")
	}
	if state.WellSortedCheck {
		good, e := c.manager.IsWellSorted(proof, nil)
		if e != nil {
			return e
		}
		if !good {
			return fmt.Errorf("proof is not well sorted")
		}
	}
	pp, err := ast.NewPPParams(c.config.Params, c.config.Globals, nil)
	if err != nil {
		return err
	}
	pretty, err := pp.PrettyProof()
	if err = errors.Join(err, pp.Close()); err != nil {
		return err
	}
	if pretty {
		err = c.WriteExpression(out, proof)
	} else {
		printer, e := ast.NewLegacySMTPrinter(c.manager)
		if e != nil {
			return e
		}
		var logic usymbol.Symbol
		if c.logic != "" {
			logic, e = c.manager.Symbols.Intern(c.logic)
			if e != nil {
				return e
			}
		}
		declaredName := func(h ast.ASTHandle) (string, error) {
			name, e := c.manager.GetASTName(h)
			if e != nil {
				return "", e
			}
			return c.manager.Symbols.String(name)
		}
		config := ast.LegacySMTBenchmarkConfig{Logic: logic,
			IsDeclaredSort: func(h ast.ASTHandle) (bool, error) {
				name, e := declaredName(h)
				_, found := c.sorts[name]
				return found, e
			},
			IsDeclaredFunc: func(h ast.ASTHandle) (bool, error) { name, e := declaredName(h); return len(c.functions[name]) != 0, e },
		}
		err = printer.DisplayBenchmark(out, proof, config)
	}
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}
