package cmd_context

// Source: src/cmd_context/cmd_context.cpp retained solver, assertion,
// declaration and scope operations; cmd_context.h.

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
	"github.com/glycerine/insdict"
)

type declaration struct {
	name   string
	handle ast.ASTHandle
	ref    ast.ASTRef
}
type assertion struct {
	expression, tag ast.ASTHandle
	ref, tagRef     ast.ASTRef
	displayText     string
}
type optimizationObjective struct {
	expression  ast.ASTHandle
	ref         ast.ASTRef
	value       smtcore.SMTArithmeticObjectiveValue
	modelValue  smtcore.SMTArithmeticObjectiveValue
	softTerms   []optimizationSoftTerm
	softValue   big.Rat
	softOffset  big.Rat
	boundLower  big.Rat
	boundUpper  big.Rat
	id          string
	maximize    bool
	soft        bool
	bitVector   bool
	bitWidth    uint32
	interval    bool
	openBounds  bool
	valueReady  bool
	boundsReady bool
}
type optimizationSoftTerm struct {
	expression ast.ASTHandle
	ref        ast.ASTRef
	weight     big.Rat
}
type scope struct{ functions, sorts, assertions, objectives, softTerms, macros, sortConstructors int }
type builtin struct {
	family ast.FamilyID
	kind   int32
}

// Context owns its AST manager, solver, declarations and original assertions.
// Parameter and model-evaluator services are borrowed unless constructed by
// NewStandaloneContext. The AST manager and solver initialize on first use,
// so set-logic and proof options take effect before their construction.
type Context struct {
	config                           smtcore.SMTSolverConfig
	parameters                       *smtcore.ContextParams
	params                           uparams.ParamsRef
	paramsReady                      bool
	manager                          *ast.ASTManager
	debugExpressions                 map[string]ast.ASTRef
	borrowedManager                  bool
	fixedpoint                       *muzfp.PropositionalEngine
	datalogDisplay                   *base.DatalogContext
	solver                           solver.Solver
	tacticResult                     *solver.SimpleCheckSatResult
	retainedSolverResult             solver.Solver
	logic                            string
	functions                        map[string][]ast.ASTHandle
	sorts                            map[string]ast.ASTHandle
	sortArities                      map[string]uint32
	sortConstructors                 map[string]sortConstructorDeclaration
	sortConstructorTrail             []string
	builtins                         map[string][]builtin
	functionTrail, sortTrail         []declaration
	macros                           map[string][]macroDeclaration
	macroTrail                       []string
	userTactics                      *insdict.Dict[string, string]
	assertions                       []assertion
	objectives                       []optimizationObjective
	optimizationHardOverride         []ast.ASTHandle
	optimizationHardOverrideActive   bool
	optimizationPreprocessor         tactic.ModelConverter
	softObjectiveIndex               map[string]int
	softObjectiveTrail               []int
	optimizationBoxNext              int
	optimizationBoxActive            bool
	optimizationBoxModels            []*model.ModelCore
	optimizationBoxValues            []opt.ObjectiveValue
	optimizationBoxLabels            []usymbol.Symbol
	optimizationBoxReasonUnknown     string
	optimizationException            string
	optimizationParetoActive         bool
	optimizationParetoBackend        *smtcore.SMTSolver
	optimizationParetoPurification   *paretoPurification
	scopes                           []scope
	PrintSuccess, GlobalDeclarations bool
	InteractiveMode                  bool
	ProduceUnsatAssumptions          bool
	ProduceAssignments               bool
	NumeralAsReal                    bool
	ExitOnError                      bool
	RandomSeed                       uint32
	checked                          bool
	status                           lbool.LBool
	progressError                    error
	infoStatus                       lbool.LBool
	watch                            stopwatch.Stopwatch
	owned                            *contextServices
	closed                           bool
	initializationError              error
	regularOutput                    io.Writer
	shellOutput                      io.Writer
	shellDiagnostic                  io.Writer
	diagnosticOutput                 io.Writer
	regularOutputName                string
	diagnosticOutputName             string
	regularFile                      *os.File
	diagnosticFile                   *os.File
	mainContext                      bool
	diagnosticBound                  bool
	diagnosticGeneration             uint64
	previousWarning                  io.Writer
	previousVerbose                  *utext.NumericTextStream
	diagnosticVerbose                *utext.NumericTextStream
	logicChecker                     LogicChecker
}

func NewContext(config smtcore.SMTSolverConfig) (*Context, error) {
	if config.Params == nil || config.Globals == nil {
		return nil, uparams.ErrParamsOwner
	}
	c := &Context{config: config, mainContext: true, functions: make(map[string][]ast.ASTHandle), sorts: make(map[string]ast.ASTHandle), sortArities: make(map[string]uint32), sortConstructors: make(map[string]sortConstructorDeclaration), builtins: make(map[string][]builtin), macros: make(map[string][]macroDeclaration), userTactics: insdict.NewDict[string, string](), softObjectiveIndex: make(map[string]int), status: lbool.Undef}
	var err error
	c.params, err = config.Params.NewValue()
	if err != nil {
		return c, err
	}
	c.paramsReady = true
	if config.ModelEvaluator.Params != nil {
		if err = config.Params.Append(&c.params, config.ModelEvaluator.Params); err != nil {
			return c, err
		}
	}
	c.config.ModelEvaluator.Params = &c.params
	c.parameters, err = smtcore.NewContextParams(config.Params, config.Globals, config.Params.BudgetField)
	if err != nil {
		return c, err
	}
	state, err := c.parameters.State()
	if err != nil {
		return c, err
	}
	c.PrintSuccess = state.SMTLib2Compliant
	return c, nil
}

func (c *Context) valid() error {
	if c == nil || c.closed || c.parameters == nil {
		return fmt.Errorf("closed command context")
	}
	return nil
}

// SetRegularOutput borrows the command session's current output transport.
// Source: cmd_context.h set_regular_stream(std::ostream&). Existing solver
// strategies resolve this writer when printing, so a later parser session can
// change the destination without reconstructing the solver.
func (c *Context) SetRegularOutput(output io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if w, ok := output.(contextRegularOutput); ok && w.context == c {
		return nil
	}
	if c.regularFile != nil {
		if err := c.regularFile.Close(); err != nil {
			return err
		}
		c.regularFile = nil
	}
	c.regularOutput = output
	c.shellOutput = nil
	c.regularOutputName = "caller-owned"
	return nil
}

// SetShellRegularOutput supplies the shell's stdout transport. It can be a
// caller-provided writer while retaining the source shell's stdout identity.
func (c *Context) SetShellRegularOutput(output io.Writer) error {
	if w, ok := output.(contextRegularOutput); ok && w.context == c {
		return c.valid()
	}
	if err := c.SetRegularOutput(output); err != nil {
		return err
	}
	c.shellOutput = output
	c.regularOutputName = "stdout"
	return nil
}

// SetShellDiagnosticOutput supplies the shell's stderr transport without
// changing the context's current channel selection.
func (c *Context) SetShellDiagnosticOutput(output io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	c.shellDiagnostic = output
	if output != nil && (c.diagnosticOutputName == "" || c.diagnosticOutputName == "stderr") {
		c.diagnosticOutput = contextShellDiagnosticOutput{c}
		if c.mainContext {
			if err := c.bindDiagnosticGlobals(); err != nil {
				return errors.Join(err, c.unbindDiagnosticGlobals())
			}
		}
	}
	return nil
}

// SetDiagnosticOutput borrows the command session's diagnostic transport.
// Source: cmd_context.h set_diagnostic_stream(std::ostream&).
func (c *Context) SetDiagnosticOutput(output io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if err := c.unbindDiagnosticGlobals(); err != nil {
		return err
	}
	if c.diagnosticFile != nil {
		if err := c.diagnosticFile.Close(); err != nil {
			return err
		}
		c.diagnosticFile = nil
	}
	c.diagnosticOutput = output
	c.diagnosticOutputName = "caller-owned"
	return nil
}

// RegularOutput follows the current stream even when a command changes it.
func (c *Context) RegularOutput() io.Writer { return contextRegularOutput{c} }

// DiagnosticOutput follows the current diagnostic stream even when a command
// changes its destination.
func (c *Context) DiagnosticOutput() io.Writer { return contextDiagnosticOutput{c} }

func (c *Context) setOutputChannel(name string, regular bool) error {
	output, channelName, file := &c.diagnosticOutput, &c.diagnosticOutputName, &c.diagnosticFile
	defaultName := "stderr"
	if regular {
		output, channelName, file = &c.regularOutput, &c.regularOutputName, &c.regularFile
		defaultName = "stdout"
	}
	if *file != nil {
		if err := (*file).Close(); err != nil {
			return err
		}
		*file = nil
	}
	*output, *channelName = nil, defaultName
	if regular && c.shellOutput != nil {
		*output = c.shellOutput
	}
	switch name {
	case "stdout":
		if c.shellOutput != nil {
			if regular {
				*output = c.shellOutput
			} else {
				*output = contextShellOutput{c}
			}
		} else {
			*output = os.Stdout
		}
	case "stderr":
		if c.shellDiagnostic != nil {
			*output = contextShellDiagnosticOutput{c}
		} else {
			*output = os.Stderr
		}
	default:
		opened, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			if !regular {
				_ = c.unbindDiagnosticGlobals()
			}
			return fmt.Errorf("failed to set output stream '%s': %w", name, err)
		}
		*output, *file = opened, opened
	}
	*channelName = name
	if !regular && c.mainContext {
		if err := c.bindDiagnosticGlobals(); err != nil {
			return errors.Join(err, c.unbindDiagnosticGlobals())
		}
	}
	return nil
}

// bindDiagnosticGlobals implements cmd_context::set_diagnostic_stream for a
// main context. The formatter borrows the selected file or shell writer.
func (c *Context) bindDiagnosticGlobals() error {
	config := utext.NumericTextStreamConfig{Budget: c.config.Params.BudgetField}
	if file, ok := c.diagnosticOutput.(*os.File); ok {
		config.BorrowedFile = file
	} else {
		config.BorrowedWriter = c.diagnosticOutput
	}
	stream, err := utext.NewNumericTextStream(config)
	if err != nil {
		return err
	}
	warnings := diagnostic.BorrowProcessWarningService()
	if !c.diagnosticBound {
		c.previousWarning = warnings.WarningStream()
		c.previousVerbose, err = diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return errors.Join(err, stream.Close())
		}
	}
	c.diagnosticGeneration++
	if err = warnings.SetWarningStream(contextWarningOutput{c, c.diagnosticGeneration}); err != nil {
		return errors.Join(err, stream.Close())
	}
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		return errors.Join(err, stream.Close())
	}
	old := c.diagnosticVerbose
	c.diagnosticVerbose, c.diagnosticBound = stream, true
	if old != nil {
		return old.Close()
	}
	return nil
}

func (c *Context) unbindDiagnosticGlobals() error {
	if !c.diagnosticBound {
		return nil
	}
	warnings := diagnostic.BorrowProcessWarningService()
	if selected, ok := warnings.WarningStream().(contextWarningOutput); ok && selected.context == c && selected.generation == c.diagnosticGeneration {
		previous := c.previousWarning
		if prior, ok := previous.(contextWarningOutput); ok && (prior.context.closed || prior.generation != prior.context.diagnosticGeneration) {
			previous = nil
		}
		if err := warnings.SetWarningStream(previous); err != nil {
			return err
		}
	}
	selected, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if selected == c.diagnosticVerbose {
		previous := c.previousVerbose
		if previous.Closed {
			previous = diagnostic.BorrowDefaultProcessVerboseStream()
		}
		if err = diagnostic.SetProcessVerboseStream(previous); err != nil {
			return err
		}
	}
	if err = c.diagnosticVerbose.Close(); err != nil {
		return err
	}
	c.diagnosticVerbose, c.previousVerbose, c.previousWarning = nil, nil, nil
	c.diagnosticBound = false
	return nil
}

type contextRegularOutput struct{ context *Context }

type contextShellOutput struct{ context *Context }

type contextShellDiagnosticOutput struct{ context *Context }

func (w contextShellDiagnosticOutput) Write(data []byte) (int, error) {
	output := w.context.shellDiagnostic
	if output == nil {
		output = os.Stderr
	}
	return output.Write(data)
}

func (w contextShellDiagnosticOutput) Flush() error { return flushOutput(w.context.shellDiagnostic) }

func (w contextShellOutput) Write(data []byte) (int, error) {
	output := w.context.shellOutput
	if output == nil {
		output = os.Stdout
	}
	return output.Write(data)
}

func (w contextShellOutput) Flush() error { return flushOutput(w.context.shellOutput) }

func (w contextRegularOutput) Write(data []byte) (int, error) {
	output := w.context.regularOutput
	if output == nil {
		output = os.Stdout
	}
	return output.Write(data)
}

type contextDiagnosticOutput struct{ context *Context }

// Each selected ostream has a distinct sticky failure state, even if its
// enclosing context subsequently opens another diagnostic destination.
type contextWarningOutput struct {
	context    *Context
	generation uint64
}

func (w contextWarningOutput) Write(data []byte) (int, error) {
	return (contextDiagnosticOutput{w.context}).Write(data)
}

func (w contextWarningOutput) Flush() error { return flushOutput(w.context.diagnosticOutput) }

func flushOutput(output io.Writer) error {
	if flusher, ok := output.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func (w contextDiagnosticOutput) Write(data []byte) (int, error) {
	output := w.context.diagnosticOutput
	if output == nil {
		output = os.Stderr
	}
	return output.Write(data)
}

func (c *Context) Compliant() (bool, error) {
	if err := c.valid(); err != nil {
		return false, err
	}
	state, err := c.parameters.State()
	return state.SMTLib2Compliant, err
}

// GlobalParameters is borrowed by the source shell's command-line scanner.
func (c *Context) GlobalParameters() (*uparams.GlobalParamsService, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	return c.config.Globals, nil
}

// RefreshGlobalParameters is reached after source command-line assignments
// and before constructing the AST manager and solver.
func (c *Context) RefreshGlobalParameters() error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.manager != nil {
		return fmt.Errorf("command context is already initialized")
	}
	if err := c.parameters.UpdateGlobal(); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err == nil {
		c.PrintSuccess = state.SMTLib2Compliant
	}
	return err
}

func (c *Context) SetLogic(logic string) error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.logic != "" {
		return fmt.Errorf("the logic has already been set")
	}
	if c.manager != nil && !c.borrowedManager {
		return fmt.Errorf("logic must be set before initialization")
	}
	if !solver.SupportedLogic(logic) {
		return fmt.Errorf("unsupported logic %s", logic)
	}
	c.logic = logic
	if c.manager != nil {
		return c.logicChecker.SetLogic(c.manager, logic)
	}
	return nil
}

func (c *Context) NumeralsAsReal() bool { return c.NumeralAsReal || solver.LogicHasRealsOnly(c.logic) }

func (c *Context) SetBoolOption(name string, value bool) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	switch name {
	case "print_success":
		c.PrintSuccess = value
		return nil
	case "interactive_mode", "produce_assertions":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		c.InteractiveMode = value
		return nil
	case "produce_unsat_assumptions":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		c.ProduceUnsatAssumptions = value
		return nil
	case "produce_assignments":
		c.ProduceAssignments = value
		return nil
	case "numeral_as_real":
		c.NumeralAsReal = value
		return nil
	case "int_real_coercions":
		manager, err := c.Manager()
		if err != nil {
			return err
		}
		return manager.EnableIntRealCoercions(value)
	case "global_declarations", "global_decls":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		c.GlobalDeclarations = value
		return nil
	case "produce_models":
		name = "model"
	case "produce_proofs":
		name = "proof"
	case "produce_interpolants":
		if c.manager != nil {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		name = "proof"
	case "produce_unsat_cores":
		name = "unsat_core"
	}
	if name == "proof" || name == "model" || name == "unsat_core" {
		if c.manager != nil && (name == "proof" || name == "unsat_core") {
			return fmt.Errorf("option value cannot be modified after initialization")
		}
		text := strconv.FormatBool(value)
		if err := c.parameters.Set(&name, &text); err != nil {
			return err
		}
	} else {
		if err := c.config.Globals.SourceSetName(name, strconv.FormatBool(value)); err != nil {
			return err
		}
		return c.globalParametersUpdated()
	}
	if err := c.config.Params.SetBoolName(&c.params, &name, value); err != nil {
		return err
	}
	if c.solver != nil {
		if err := c.solver.UpdateParams(&c.params); err != nil {
			return err
		}
		if name == "model" {
			return c.solver.SetProduceModels(value)
		}
	}
	return nil
}

// SetUIntOption follows the retained unsigned front-end and module option
// paths. The source parameter catalog validates module names and types.
func (c *Context) SetUIntOption(name string, value uint32) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if name == "random_seed" {
		c.RandomSeed = value
		return nil
	}
	if name == "verbosity" {
		return diagnostic.SetProcessVerbosityLevel(value)
	}
	if name == "reproducible_resource_limit" {
		name = "rlimit"
	}
	text := strconv.FormatUint(uint64(value), 10)
	if name == "timeout" || name == "rlimit" {
		if err := c.parameters.Set(&name, &text); err != nil {
			return err
		}
		if err := c.config.Params.SetUIntName(&c.params, &name, value); err != nil {
			return err
		}
	} else {
		if err := c.config.Globals.SourceSetName(name, text); err != nil {
			return err
		}
		// Preserve the catalog's type. An integer token can also supply a
		// Double parameter; a local UInt shadow would hide that typed value.
		return c.globalParametersUpdated()
	}
	if c.solver != nil {
		return c.solver.UpdateParams(&c.params)
	}
	return nil
}

// SetDoubleOption follows the catalog-typed global parameter path. Floating
// point parameters are solver tunables rather than SMT-LIB context options.
func (c *Context) SetDoubleOption(name string, value float64) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if err := c.config.Globals.SourceSetName(name, strconv.FormatFloat(value, 'g', -1, 64)); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

// SetNumeralOption applies the source set_option_cmd rational-value branch.
// Generic module parameters retain their catalog type; the three numeric
// builtins are converted through the source unsigned-machine-integer path.
func (c *Context) SetNumeralOption(name string, value *big.Rat) error {
	if err := c.valid(); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("invalid option value")
	}
	name = strings.ReplaceAll(name, "-", "_")
	switch name {
	case "random_seed", "verbosity", "reproducible_resource_limit", "timeout", "rlimit":
		if !value.IsInt() || value.Sign() < 0 || !value.Num().IsUint64() || value.Num().Uint64() > uint64(^uint32(0)) {
			return fmt.Errorf("option value is too big to fit in a machine integer.")
		}
		return c.SetUIntOption(name, uint32(value.Num().Uint64()))
	case "print_success", "print_warning", "expand_definitions", "interactive_mode", "produce_proofs", "produce_unsat_cores", "produce_unsat_assumptions", "produce_models", "produce_assignments", "produce_interpolants", "regular_output_channel", "diagnostic_output_channel", "global_decls", "global_declarations", "produce_assertions":
		return fmt.Errorf("option value is not a numeral")
	}
	text := rationalOptionText(value)
	if err := c.config.Globals.SourceSetName(name, text); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

func rationalOptionText(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	denominator := new(big.Int).Set(value.Denom())
	two, five := 0, 0
	for new(big.Int).Mod(denominator, big.NewInt(2)).Sign() == 0 {
		denominator.Div(denominator, big.NewInt(2))
		two++
	}
	for new(big.Int).Mod(denominator, big.NewInt(5)).Sign() == 0 {
		denominator.Div(denominator, big.NewInt(5))
		five++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return value.RatString()
	}
	places := two
	if five > places {
		places = five
	}
	return value.FloatString(places)
}

// SetSymbolOption follows set_option_cmd::set_symbol's generic parameter
// path. The source catalog validates the module/name and parses its type;
// keeping that typed global value avoids an incorrectly typed local shadow.
func (c *Context) SetSymbolOption(name, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if name == "error_behavior" {
		switch value {
		case "immediate-exit":
			c.ExitOnError = true
		case "continued-execution":
			c.ExitOnError = false
		default:
			return fmt.Errorf("error setting :error-behavior, 'immediate-execution' or 'continued-execution' expected")
		}
		return nil
	}
	if err := c.config.Globals.SourceSetName(name, value); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

// SetStringOption follows the context-local string and output-channel paths.
func (c *Context) SetStringOption(name, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if name == "regular_output_channel" || name == "diagnostic_output_channel" {
		return c.setOutputChannel(value, name == "regular_output_channel")
	}
	if name == "dot_proof_file" || name == "trace_file_name" {
		if err := c.parameters.Set(&name, &value); err != nil {
			return err
		}
		return c.config.Params.SetStringName(&c.params, &name, &value)
	}
	if err := c.config.Globals.SourceSetName(name, value); err != nil {
		return err
	}
	return c.globalParametersUpdated()
}

// UpdateParamValue is the context_params::set path used by the external API.
// Unlike SMT-LIB set-option, it updates only this context's construction
// parameters and leaves the process-global parameter service unchanged.
func (c *Context) UpdateParamValue(name, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	return c.parameters.Set(&name, &value)
}

// Source cmd_context::global_params_updated. Generic option values retain
// their catalog types in gparams. Only the disabled context auto_config flag
// is copied into the solver's local update, as in the native command context.
func (c *Context) globalParametersUpdated() (result error) {
	global, err := c.config.Globals.Manager()
	if err != nil {
		return err
	}
	if err = uparams.UpdateProcessEnvParams(global); err != nil {
		return err
	}
	if err := c.parameters.UpdateGlobal(); err != nil {
		return err
	}
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if state.SMTLib2Compliant {
		c.PrintSuccess = true
	}
	if c.fixedpoint != nil {
		if err := c.fixedpoint.ParametersUpdated(); err != nil {
			return err
		}
	}
	if c.solver == nil {
		return nil
	}
	params := c.config.Params.EmptyValue()
	defer func() { result = errors.Join(result, c.config.Params.Del(&params)) }()
	if !state.AutoConfig {
		name := "auto_config"
		if err := c.config.Params.SetBoolName(&params, &name, false); err != nil {
			return err
		}
	}
	return c.solver.UpdateParams(&params)
}

func (c *Context) Manager() (manager *ast.ASTManager, result error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if c.initializationError != nil {
		return nil, c.initializationError
	}
	defer func() {
		if result != nil {
			c.initializationError = result
		}
	}()
	if c.manager != nil {
		return c.manager, nil
	}
	p := c.config.Params
	m, err := c.parameters.MkASTManager(ast.ASTManagerConfig{Symbols: p.SymbolsField, Numerals: p.NumeralsField, Budget: p.BudgetField})
	c.manager = m
	if err != nil {
		return nil, err
	}
	if err = m.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	if err = c.initBuiltins(); err != nil {
		return nil, err
	}
	if err = c.mkSolver(m); err != nil {
		return nil, err
	}
	if err = c.logicChecker.SetLogic(m, c.logic); err != nil {
		return nil, err
	}
	return m, nil
}

// mkSolver mirrors the shell factory step independently of manager creation.
func (c *Context) mkSolver(m *ast.ASTManager) error {
	var err error
	var proofs, models, cores bool
	if err = c.parameters.GetSolverParams(m, &c.params, &proofs, &models, &cores); err != nil {
		return err
	}
	switch c.logic {
	case "QF_FD", "SAT":
		enabled, e := m.ProofsEnabled()
		if e != nil {
			return e
		}
		if !enabled {
			c.solver, err = portfolio.NewFDSolverWithOutput(m, &c.params, c.config.ModelEvaluator, contextRegularOutput{c})
		} else {
			var direct *smtcore.SMTSolver
			direct, err = smtcore.NewSMTSolver(m, &c.params, c.logic, c.config)
			if err == nil {
				c.solver = direct
			}
		}
	case "UFNIA":
		c.solver, err = portfolio.NewUFNIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "UFLRA":
		c.solver, err = portfolio.NewUFLRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "AUFLIA":
		c.solver, err = portfolio.NewAUFLIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "AUFLIRA":
		c.solver, err = portfolio.NewAUFLIRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "AUFNIRA":
		c.solver, err = portfolio.NewAUFNIRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_AUFLIA":
		c.solver, err = portfolio.NewQFAUFLIAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_LRA":
		c.solver, err = portfolio.NewQFLRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_NRA":
		c.solver, err = portfolio.NewQFNRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "NRA":
		c.solver, err = portfolio.NewNRAStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_FP", "QF_FPBV", "QF_BVFP", "QF_FPLRA":
		c.solver, err = portfolio.NewQFFPStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.logic, c.config, contextRegularOutput{c})
	case "QF_NIA":
		c.solver, err = portfolio.NewQFNIAStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_UF":
		c.solver, err = portfolio.NewQFUFStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_IDL":
		c.solver, err = portfolio.NewQFIDLStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_LIA":
		c.solver, err = portfolio.NewQFLIAStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_BV":
		c.solver, err = portfolio.NewQFBVStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "QF_AUFBV":
		c.solver, err = portfolio.NewQFAUFBVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_ABV":
		c.solver, err = portfolio.NewQFABVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "QF_UFBV":
		c.solver, err = portfolio.NewQFUFBVStrategicSolverWithOutput(m, &c.params, proofs, models, cores, c.config, contextRegularOutput{c})
	case "UFBV":
		c.solver, err = portfolio.NewUFBVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "BV":
		c.solver, err = portfolio.NewBVStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "HORN":
		c.solver, err = portfolio.NewHornStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	case "ALL", "QF_S":
		c.solver, err = portfolio.NewALLStrategicSolver(m, &c.params, proofs, models, cores, c.config)
	default:
		var direct *smtcore.SMTSolver
		direct, err = smtcore.NewSMTSolver(m, &c.params, c.logic, c.config)
		if err == nil {
			c.solver = direct
		}
	}
	if err != nil {
		return err
	}
	if err = c.solver.SetProduceModels(models); err != nil {
		return err
	}
	return nil
}

func (c *Context) DeclareSort(name string) (ast.ASTHandle, error) {
	return c.DeclareSortConstructor(name, 0)
}

func (c *Context) LookupSort(name string) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if declaration, found := c.sortConstructors[name]; found {
		return ast.ASTHandle{}, fmt.Errorf("sort constructor expects %d parameters", declaration.arity)
	}
	if s, found := c.sorts[name]; found {
		if arity := c.sortArities[name]; arity != 0 {
			return ast.ASTHandle{}, fmt.Errorf("sort constructor expects %d parameters", arity)
		}
		return s, nil
	}
	if !c.BuiltinSortAvailable(name) {
		return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
	}
	if name == "FloatingPoint" {
		return ast.ASTHandle{}, fmt.Errorf("invalid floating-point sort, expecting two parameters")
	}
	for i, alias := range []string{"RoundingMode", "Float16", "Float32", "Float64", "Float128"} {
		if name == alias {
			family, err := m.RegisterFloatingPointPlugin()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkFamilySort(family, int32(i+1), nil)
		}
	}
	if name == "Seq" {
		return ast.ASTHandle{}, fmt.Errorf("Invalid sequence sort, expecting one parameter")
	}
	if name == "RegEx" {
		return ast.ASTHandle{}, fmt.Errorf("Invalid regex sort, expecting one parameter")
	}
	if name == "Bool" {
		return m.MkBoolSort()
	}
	if name == "String" || name == "StringSequence" {
		family, err := m.SequenceFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return m.MkFamilySort(family, 2, nil)
	}
	if name == "Int" || name == "Real" {
		family, err := m.ArithmeticFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		kind := int32(0)
		if name == "Int" {
			kind = 1
		}
		return m.MkFamilySort(family, kind, nil)
	}
	return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
}

func (c *Context) DeclareFunction(name string, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := m.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.publishFunctionDeclaration(name, d)
}

func (c *Context) publishFunctionDeclaration(name string, d ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arity, err := m.GetArity(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		domain[i], err = m.GetDomain(d, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	ref, err := m.NewRef(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, m.DelRef(&ref))
		}
	}()
	if err = c.logicChecker.CheckDeclaration(d); err != nil {
		return ast.ASTHandle{}, err
	}
	if c.containsMacro(name, domain) {
		return ast.ASTHandle{}, fmt.Errorf("invalid declaration, named expression already defined with this name %s", name)
	}
	if _, found := c.builtins[name]; found {
		return ast.ASTHandle{}, fmt.Errorf("invalid declaration, builtin symbol %s", name)
	}
	for _, prior := range c.functions[name] {
		if prior == d {
			return ast.ASTHandle{}, fmt.Errorf("invalid declaration, '%s' (with the given signature) already declared", name)
		}
	}
	c.functions[name] = append(c.functions[name], d)
	c.functionTrail = append(c.functionTrail, declaration{name, d, ref})
	published = true
	return d, nil
}

func (c *Context) Assert(formula ast.ASTHandle, name string) (result error) {
	return c.AssertWithText(formula, name, "")
}

func (c *Context) AssertWithText(formula ast.ASTHandle, name, displayText string) (result error) {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if err = c.logicChecker.CheckExpression(formula); err != nil {
		return err
	}
	if err = c.invalidateOptimizationState(); err != nil {
		return err
	}
	ref, err := m.NewRef(formula)
	if err != nil {
		return err
	}
	row := assertion{expression: formula, ref: ref, displayText: displayText}
	state, err := c.parameters.State()
	if err == nil && name != "" && state.UnsatCore {
		symbol, e := m.Symbols.Intern(name)
		err = e
		if err == nil {
			b, e := m.MkBoolSort()
			err = e
			if err == nil {
				row.tag, err = m.MkConst(symbol, b)
			}
		}
		if err == nil {
			row.tagRef, err = m.NewRef(row.tag)
		}
	}
	if err == nil {
		if row.tag.IsNull() {
			err = c.solver.Assert(formula)
		} else {
			err = c.solver.AssertTracked(formula, row.tag)
		}
	}
	if err != nil {
		if row.tagRef != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&row.tagRef))
		}
		return errors.Join(err, m.DelRef(&row.ref))
	}
	c.assertions = append(c.assertions, row)
	c.checked = false
	return nil
}

func (c *Context) Push(count uint32) error {
	if count == 0 {
		return c.valid()
	}
	c.checked = false
	if err := c.clearCheckSatResult(); err != nil {
		return err
	}
	if _, err := c.Manager(); err != nil {
		return err
	}
	for ; count > 0; count-- {
		if err := c.solver.Push(); err != nil {
			return err
		}
		c.scopes = append(c.scopes, scope{len(c.functionTrail), len(c.sortTrail), len(c.assertions), len(c.objectives), len(c.softObjectiveTrail), len(c.macroTrail), len(c.sortConstructorTrail)})
	}
	c.checked = false
	return nil
}

func (c *Context) Pop(count uint32) error {
	if err := c.valid(); err != nil {
		return err
	}
	c.checked = false
	if err := c.clearCheckSatResult(); err != nil {
		return err
	}
	if uint64(count) > uint64(len(c.scopes)) {
		return fmt.Errorf("invalid pop command, argument is greater than the current stack depth")
	}
	if count == 0 {
		return c.invalidateOptimizationState()
	}
	if err := c.solver.Pop(count); err != nil {
		return err
	}
	if err := c.invalidateOptimizationState(); err != nil {
		return err
	}
	level := len(c.scopes) - int(count)
	s := c.scopes[level]
	if !c.GlobalDeclarations {
		if err := c.restoreMacros(s.macros); err != nil {
			return err
		}
		if err := c.restoreDeclarations(s.functions, s.sorts); err != nil {
			return err
		}
		if err := c.restoreSortConstructors(s.sortConstructors); err != nil {
			return err
		}
	}
	if err := c.restoreAssertions(s.assertions); err != nil {
		return err
	}
	if err := c.restoreObjectives(s.objectives, s.softTerms); err != nil {
		return err
	}
	c.scopes = c.scopes[:level]
	return nil
}

func (c *Context) restoreDeclarations(functions, sorts int) error {
	for len(c.functionTrail) > functions {
		i := len(c.functionTrail) - 1
		row := &c.functionTrail[i]
		values := c.functions[row.name]
		if len(values) == 1 {
			delete(c.functions, row.name)
		} else {
			c.functions[row.name] = values[:len(values)-1]
		}
		if err := c.manager.DelRef(&row.ref); err != nil {
			return err
		}
		c.functionTrail = c.functionTrail[:i]
	}
	for len(c.sortTrail) > sorts {
		i := len(c.sortTrail) - 1
		row := &c.sortTrail[i]
		delete(c.sorts, row.name)
		delete(c.sortArities, row.name)
		if err := c.manager.DelRef(&row.ref); err != nil {
			return err
		}
		c.sortTrail = c.sortTrail[:i]
	}
	return nil
}

func (c *Context) restoreAssertions(size int) error {
	for len(c.assertions) > size {
		i := len(c.assertions) - 1
		row := &c.assertions[i]
		if row.tagRef != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&row.tagRef); err != nil {
				return err
			}
		}
		if err := c.manager.DelRef(&row.ref); err != nil {
			return err
		}
		c.assertions = c.assertions[:i]
	}
	return nil
}

func (c *Context) CheckSat(assumptions []ast.ASTHandle) (status lbool.LBool, err error) {
	if _, err := c.Manager(); err != nil {
		return lbool.Undef, err
	}
	watch, err := stopwatch.NewScopedWatch(&c.watch, true)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, watch.Close()) }()
	optimizationContinuation := c.checked
	c.checked = false
	c.progressError = nil
	if err := c.clearCheckSatResult(); err != nil {
		return lbool.Undef, err
	}
	if len(c.objectives) != 0 {
		status, err = c.checkOptimize(assumptions, optimizationContinuation)
		if err != nil {
			return lbool.Undef, err
		}
	} else {
		if err = c.clearOptimizationBoxModels(); err != nil {
			return lbool.Undef, err
		}
		c.progressError = nil
		if err = c.solver.SetProgressCallback(c); err != nil {
			return lbool.Undef, err
		}
		status, err = c.solver.CheckSat(assumptions)
		if err != nil {
			var fatal z3exception.Z3Error
			var source z3exception.Z3Exception
			if errors.As(err, &fatal) || !errors.As(err, &source) {
				return lbool.Undef, err
			}
			// Native cmd_context converts nonfatal solver exceptions into an
			// unknown result owned by the installed solver. In particular, this
			// prevents combined_solver from treating a malformed model value as
			// an ordinary solver-2 unknown and falling back to solver 1.
			status = lbool.Undef
			if setErr := c.solver.SetReasonUnknown(source.Message()); setErr != nil {
				return lbool.Undef, errors.Join(err, setErr)
			}
			if setErr := c.solver.SetStatus(status); setErr != nil {
				return lbool.Undef, errors.Join(err, setErr)
			}
			err = nil
		}
		c.status, c.checked = status, true
		if c.progressError != nil {
			return status, c.progressError
		}
	}
	if status == lbool.True {
		if err = c.completeModel(); err != nil {
			return status, err
		}
		if err = c.validateModel(); err != nil {
			return status, err
		}
	}
	return status, err
}

func (c *Context) Model() (*model.ModelCore, error) {
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	if !c.checked || c.status != lbool.True && c.status != lbool.Undef || !state.Model {
		return nil, fmt.Errorf("model is not available")
	}
	value, err := c.checkSatResult().AcquireModel()
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("model is not available")
	}
	return value, nil
}

func (c *Context) WriteModel(out io.Writer) (result error) {
	return c.WriteModelAtModelIndex(out, 0)
}

// WriteModelAtModelIndex matches get-model's optional box-objective index.
// Without an optimizer, the source command falls back to the current model.
func (c *Context) WriteModelAtModelIndex(out io.Writer, modelIndex uint32) (result error) {
	m, err := c.modelAtIndex(modelIndex, true)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DecRef()) }()
	if _, err = io.WriteString(out, "(model \n"); err != nil {
		return err
	}
	if err = m.WriteSMT2(out, c.config.Params, c.config.Globals, 2); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}

func (c *Context) modelAtIndex(modelIndex uint32, fallbackWithoutOptimizer bool) (*model.ModelCore, error) {
	current, err := c.Model()
	if err != nil || modelIndex == 0 {
		return current, err
	}
	if len(c.objectives) == 0 && fallbackWithoutOptimizer {
		return current, nil
	}
	if err = current.DecRef(); err != nil {
		return nil, err
	}
	if uint64(modelIndex) >= uint64(len(c.optimizationBoxModels)) {
		return nil, fmt.Errorf("index into models is out of bounds")
	}
	selected := c.optimizationBoxModels[modelIndex]
	if selected == nil {
		return nil, fmt.Errorf("model is not available")
	}
	if err = selected.IncRef(); err != nil {
		return nil, err
	}
	return selected, nil
}

// ResetAssertions follows cmd_context::reset_assertions. With global
// declarations disabled, source performs a complete owning-session reset.
// Otherwise the declaration world and scope stack survive solver recreation.
func (c *Context) ResetAssertions() error {
	if err := c.valid(); err != nil {
		return err
	}
	if !c.GlobalDeclarations {
		return c.Reset()
	}
	// Source m_check_sat_result owns its previous solver independently of the
	// replacement m_solver. Keep that result live until normal invalidation.
	if !c.checked {
		if err := c.clearCheckSatResult(); err != nil {
			return err
		}
	}
	if c.solver != nil {
		if c.checked && c.tacticResult == nil && c.retainedSolverResult == nil {
			c.retainedSolverResult = c.solver
		} else if err := c.solver.Close(); err != nil {
			return err
		}
		c.solver = nil
		if err := c.mkSolver(c.manager); err != nil {
			c.initializationError = err
			return err
		}
	}
	if err := c.restoreAssertions(0); err != nil {
		return err
	}
	if err := c.restoreObjectives(0, 0); err != nil {
		return err
	}
	for i := range c.scopes {
		c.scopes[i].assertions = 0
		c.scopes[i].objectives = 0
		c.scopes[i].softTerms = 0
		if c.solver != nil {
			if err := c.solver.Push(); err != nil {
				c.initializationError = err
				return err
			}
		}
	}
	return nil
}

// Reset follows cmd_context::reset(false) for this owning command context.
// Session options, output and parameter services survive; the AST manager is
// rebuilt lazily after all old-manager consumers have been released.
func (c *Context) Reset() error {
	if err := c.valid(); err != nil {
		return err
	}
	if !c.borrowedManager {
		return c.resetManagerState()
	}
	manager := c.manager
	if err := c.resetManagerState(); err != nil {
		return err
	}
	c.manager = manager
	if err := c.initBuiltins(); err != nil {
		return err
	}
	return c.mkSolver(manager)
}

// Also used by Close, including partial context construction where valid()
// cannot succeed yet. Parameter services belong to the enclosing context.
func (c *Context) resetManagerState() error {
	var construction *portfolio.FDConstructionFailure
	if errors.As(c.initializationError, &construction) && construction.CleanupPending() {
		return construction
	}
	if err := c.closeParetoBackend(); err != nil {
		return err
	}
	if err := c.clearOptimizationBoxModels(); err != nil {
		return err
	}
	if err := c.clearCheckSatResult(); err != nil {
		return err
	}
	if c.fixedpoint != nil {
		if err := c.fixedpoint.Close(); err != nil {
			return err
		}
		c.fixedpoint = nil
	}
	if c.datalogDisplay != nil {
		if _, err := c.manager.ProofMode(); err != nil {
			return err
		}
		if err := c.datalogDisplay.Close(); err != nil {
			return err
		}
		c.datalogDisplay = nil
	}
	if c.solver != nil {
		if err := c.solver.Close(); err != nil {
			return err
		}
		c.solver = nil
	}
	if c.manager != nil {
		if err := c.clearDebugExpressions(); err != nil {
			return err
		}
		c.logicChecker.Reset()
		if err := c.restoreMacros(0); err != nil {
			return err
		}
		if err := c.restoreAssertions(0); err != nil {
			return err
		}
		if err := c.restoreObjectives(0, 0); err != nil {
			return err
		}
		if err := c.restoreDeclarations(0, 0); err != nil {
			return err
		}
		if err := c.restoreSortConstructors(0); err != nil {
			return err
		}
		if !c.borrowedManager {
			if err := c.manager.Close(); err != nil {
				return err
			}
		}
		c.manager = nil
	}

	clear(c.builtins)
	clear(c.sortArities)
	clear(c.sortConstructors)
	clear(c.softObjectiveIndex)
	c.softObjectiveTrail = nil
	c.sortConstructorTrail = nil
	c.logicChecker.Reset()
	c.logic = ""
	c.NumeralAsReal = false
	c.scopes = nil
	c.checked = false
	c.status = lbool.Undef
	c.initializationError = nil
	return nil
}

func (c *Context) Close() error {
	if c == nil {
		return nil
	}
	if c.closed {
		return nil
	}
	if err := c.unbindDiagnosticGlobals(); err != nil {
		return err
	}
	if err := c.resetManagerState(); err != nil {
		return err
	}
	if c.userTactics != nil {
		c.userTactics.DeleteAll()
		c.userTactics = nil
	}
	if c.parameters != nil {
		if err := c.parameters.Close(); err != nil {
			return err
		}
		c.parameters = nil
	}
	if c.paramsReady {
		if err := c.config.Params.Del(&c.params); err != nil {
			return err
		}
		c.paramsReady = false
	}
	if c.owned != nil {
		if err := c.owned.Close(); err != nil {
			return err
		}
		c.owned = nil
	}
	if c.regularFile != nil {
		if err := c.regularFile.Close(); err != nil {
			return err
		}
		c.regularFile = nil
	}
	if c.diagnosticFile != nil {
		if err := c.diagnosticFile.Close(); err != nil {
			return err
		}
		c.diagnosticFile = nil
	}
	c.closed = true
	return nil
}
