package api

// Source: retained public lifecycle of src/api/api_context.cpp and
// src/api/api_solver.cpp. Go exposes the concrete owning solver context rather
// than reproducing opaque C handles or foreign-language bindings.

import (
	"errors"
	"io"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine" // generated parameter collectors
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrSMTContextOwner = errors.New("api: invalid or closed SMT context")

// SMTContext is an owning, single-goroutine Go solver context. Copies share
// the same state; closing any copy closes the context. Manager returns the
// borrowed AST owner used to construct formulas. Keep caller-owned ASTRefs and
// model references within their documented lifetimes and release them before
// Close.
type SMTContext struct{ *smtContextState }

type smtContextState struct {
	command *cmd_context.Context
	closed  bool
}

// NewSMTContext constructs an independent solver context with source defaults.
// Logic and proof/core options must be selected before Manager initializes the
// AST manager, matching the native context lifecycle.
func NewSMTContext() (*SMTContext, error) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return nil, err
	}
	return &SMTContext{smtContextState: &smtContextState{command: command}}, nil
}

func (c *SMTContext) open() error {
	if c == nil || c.smtContextState == nil || c.closed || c.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

// Manager returns the borrowed AST manager for term construction and
// inspection.
func (c *SMTContext) Manager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.Manager()
}

func (c *SMTContext) SetLogic(logic string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetLogic(logic)
}

func (c *SMTContext) SetBoolOption(name string, value bool) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetBoolOption(name, value)
}

func (c *SMTContext) SetUIntOption(name string, value uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetUIntOption(name, value)
}

func (c *SMTContext) SetDoubleOption(name string, value float64) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetDoubleOption(name, value)
}

func (c *SMTContext) SetSymbolOption(name, value string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetSymbolOption(name, value)
}

// Assert adds an untracked Boolean formula.
func (c *SMTContext) Assert(formula ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.Assert(formula, "")
}

// AssertTracked adds a Boolean formula with a named unsat-core label.
func (c *SMTContext) AssertTracked(formula ast.ASTHandle, name string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.Assert(formula, name)
}

// Maximize adds an arithmetic or bit-vector optimization objective.
func (c *SMTContext) Maximize(expression ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddObjective(expression, true)
}

// Minimize adds an arithmetic or bit-vector optimization objective.
func (c *SMTContext) Minimize(expression ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddObjective(expression, false)
}

// AssertSoft adds a weighted Boolean constraint. Constraints with the same
// identifier belong to one soft-objective group. The weight is copied.
func (c *SMTContext) AssertSoft(formula ast.ASTHandle, weight *big.Rat, id string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddSoftConstraint(formula, weight, id)
}

// WriteObjectives writes the objective values from the latest optimal result.
func (c *SMTContext) WriteObjectives(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.WriteObjectives(out)
}

// Assertions returns the borrowed asserted formulas in insertion order.
func (c *SMTContext) Assertions() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.Assertions()
}

func (c *SMTContext) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.Push(1)
}

func (c *SMTContext) Pop(count uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.Pop(count)
}

func (c *SMTContext) ScopeLevel() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.command.ScopeLevel()
}

// CheckSat checks the current assertions under temporary assumptions.
func (c *SMTContext) CheckSat(assumptions ...ast.ASTHandle) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.command.CheckSat(assumptions)
}

// CheckSatUsing checks the current assertions with a retained built-in tactic.
// The tactic is local to this call; its model, proof, core, reason, and
// statistics are published as the context's latest result.
func (c *SMTContext) CheckSatUsing(name string) (status lbool.LBool, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	selected, err := c.command.NewBuiltinTactic(name)
	if err != nil {
		return lbool.Undef, err
	}
	if err = selected.IncRef(); err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, selected.DecRef()) }()
	return c.command.CheckSatUsing(selected, nil)
}

// Model returns one owning reference. The caller must call DecRef.
func (c *SMTContext) Model() (*model.ModelCore, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.Model()
}

// Evaluate evaluates an expression in the latest model and returns one owning
// AST reference. The caller must release it through Manager.DelRef.
func (c *SMTContext) Evaluate(expression ast.ASTHandle, completion bool) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.Evaluate(expression, completion)
}

// UnsatCore returns borrowed tracked labels from the most recent result.
func (c *SMTContext) UnsatCore() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.UnsatCore()
}

// Proof returns a borrowed proof from the most recent result.
func (c *SMTContext) Proof() (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.command.Proof()
}

func (c *SMTContext) ReasonUnknown() (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	return c.command.ReasonUnknown()
}

// Statistics returns an owning Go copy of the current solver statistics.
func (c *SMTContext) Statistics() (statistics.Statistics, error) {
	if err := c.open(); err != nil {
		return statistics.Statistics{}, err
	}
	return c.command.Statistics()
}

// Consequences returns owning AST references for consequences over variables.
// The caller must release every returned reference through Manager.DelRef.
// As in the native solver API, the query replaces the solver's internal check
// state; prior model/core/proof handles must not be requested afterward.
func (c *SMTContext) Consequences(assumptions, variables []ast.ASTHandle) (status lbool.LBool, result []ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, nil, err
	}
	status, expressions, err := c.command.Consequences(assumptions, variables)
	if err != nil {
		return status, nil, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		for i := range result {
			resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
		}
		result = nil
	}()
	result = make([]ast.ASTRef, 0, len(expressions))
	for _, expression := range expressions {
		ref, refErr := manager.NewRef(expression)
		if refErr != nil {
			return lbool.Undef, result, refErr
		}
		result = append(result, ref)
	}
	return status, result, nil
}

// ImpliedEqualities returns a class identifier for each term. Terms have the
// same identifier exactly when the current assertions imply their equality.
// The query replaces the solver's internal check state.
func (c *SMTContext) ImpliedEqualities(terms []ast.ASTHandle) (lbool.LBool, []uint32, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return c.command.ImpliedEqualities(terms)
}

// AssertSMT2 parses an SMT-LIB stream and adds its final assertion set. Check
// commands are parsed but not executed, matching the native solver-from-stream
// API. Parsing is transactional: malformed input adds no assertions.
func (c *SMTContext) AssertSMT2(input io.Reader) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	target, err := c.command.Manager()
	if err != nil {
		return err
	}
	parsed, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, parsed.Close()) }()
	if err = smt2.ParseAssertions(parsed, input); err != nil {
		return err
	}
	source, err := parsed.Manager()
	if err != nil {
		return err
	}
	translator, err := ast.NewASTTranslation(source, target, true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, translator.Close()) }()
	assertions, err := parsed.Assertions()
	if err != nil {
		return err
	}
	translated := make([]ast.ASTRef, 0, len(assertions))
	defer func() {
		for i := range translated {
			result = errors.Join(result, target.DelRef(&translated[i]))
		}
	}()
	for _, assertion := range assertions {
		formula, translateErr := translator.Translate(assertion)
		if translateErr != nil {
			return translateErr
		}
		ref, refErr := target.NewRef(formula)
		if refErr != nil {
			return refErr
		}
		translated = append(translated, ref)
	}
	for i := range translated {
		formula, refErr := target.RefGet(translated[i])
		if refErr != nil {
			return refErr
		}
		if err = c.command.Assert(formula, ""); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteSMT2 executes a complete SMT-LIB command stream against this context
// and writes command responses to output. Declarations, assertions, scopes and
// results remain available to subsequent API calls and script executions.
func (c *SMTContext) ExecuteSMT2(input io.Reader, output io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if input == nil || output == nil {
		return ErrSMTContextOwner
	}
	return smt2.ParseCommands(c.command, input, output)
}

// WriteSMT2 exports the complete current declaration/assertion scope state as
// a replayable SMT-LIB command stream. It deliberately omits check-sat, so a
// consumer may append diagnostics or continue mutating the reconstructed
// context.
func (c *SMTContext) WriteSMT2(output io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if output == nil {
		return ErrSMTContextOwner
	}
	return c.command.WriteReplaySMT2(output)
}

// SMT2 returns the replay stream produced by WriteSMT2.
func (c *SMTContext) SMT2() (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	if err := c.command.WriteReplaySMT2(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (c *SMTContext) ResetAssertions() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.ResetAssertions()
}

// Close releases the solver, AST manager, and all private runtime services.
func (c *SMTContext) Close() error {
	if c == nil || c.smtContextState == nil || c.closed {
		return nil
	}
	if err := c.command.Close(); err != nil {
		return err
	}
	c.command = nil
	c.closed = true
	return nil
}
