package fp

// Source: src/muz/ddnf/ddnf.cpp ddnf::imp::query, init_ctx, and dump_rules.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/ddnf"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SetDDNFOutput selects the borrowed writer used by the source DDNF rule dump.
// A nil writer restores the source default, stdout.
func (e *PropositionalEngine) SetDDNFOutput(out io.Writer) error {
	if err := e.open(); err != nil {
		return err
	}
	e.ddnf.output = out
	return nil
}

func (e *PropositionalEngine) queryDDNFPredicate(declaration ast.ASTHandle) (lbool.LBool, error) {
	arities, err := e.manager.GetArity(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	arguments := make([]ast.ASTHandle, arities)
	for index := uint32(0); index < arities; index++ {
		sort, getErr := e.manager.GetDomain(declaration, index)
		if getErr != nil {
			return lbool.Undef, getErr
		}
		arguments[index], err = e.manager.MkVar(arities-index-1, sort)
		if err != nil {
			return lbool.Undef, err
		}
	}
	query, err := e.manager.MkApp(declaration, arguments)
	if err != nil {
		return lbool.Undef, err
	}
	return e.QueryExpression(query)
}

func (e *PropositionalEngine) queryDDNF() (result lbool.LBool, resultErr error) {
	if e.ddnf.registry == nil {
		e.ddnf.registry = ddnf.NewRegistry(arena.VectorConfig{Descriptors: arena.Config{Budget: e.manager.Budget()}})
	}
	preprocessor, err := ddnf.NewPreprocessor(e.manager, e.ddnf.registry)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessor.Close()) }()
	output := e.ddnf.output
	if output == nil {
		output = os.Stdout
	}
	ok, err := preprocessor.PreprocessRulesWithDiagnostic(e.set, e.rules, output)
	if err != nil || !ok {
		if err != nil {
			return lbool.Undef, fmt.Errorf("ddnf preprocess: %w", err)
		}
		return lbool.Undef, nil
	}
	compiler, err := ddnf.NewCompiler(preprocessor, e.rules)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		// Source m_cache is reset per query, but m_trail owns every cached
		// expression until the DDNF engine itself is destroyed.
		e.ddnf.retainTrail(compiler.DetachReferences())
		resultErr = errors.Join(resultErr, compiler.Close())
	}()
	compiled, err := compiler.CompileRuleSet(e.set)
	if err != nil {
		return lbool.Undef, fmt.Errorf("ddnf compile: %w", err)
	}
	// The formatter may return a native cleanup owner. Drain it before the
	// compiled rule set, preserving both scopes if cleanup has to retry.
	attempt := &contextFormulaAttempt{manager: e.manager, sets: []*base.RuleSet{compiled}}
	defer func() { resultErr = attempt.finish(resultErr) }()
	if ok, err = compiled.Finalize(); err != nil {
		return lbool.Undef, err
	} else if !ok {
		return lbool.Undef, errors.New("Negation is not stratified!")
	}
	config, err := e.ddnfContextSMT2Config()
	if err != nil {
		return lbool.Undef, err
	}
	if err = compiled.WriteContextSMT2(output, config); err != nil {
		return lbool.Undef, fmt.Errorf("ddnf dump: %w", err)
	}
	// Source execute_rules is disabled; query deliberately remains unknown.
	return lbool.Undef, nil
}

// The source inner context sets only engine=datalog. Its print parameters
// fall back to the fixedpoint module; outer local overrides and background
// assertions are not copied into it (ddnf.cpp:498-504, 664-676).
func (e *PropositionalEngine) ddnfContextSMT2Config() (config base.ContextSMT2Config, resultErr error) {
	config = base.DefaultContextSMT2Config()
	if e.params == nil || e.globals == nil {
		return config, nil
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return config, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	parameters.local = nil
	config.FixedpointExtensions, err = parameters.Bool("print_fixedpoint_extensions", true)
	if err != nil {
		return config, err
	}
	config.LowLevel, err = parameters.Bool("print_low_level_smt2", false)
	if err != nil {
		return config, err
	}
	config.DeclareVariables, err = parameters.Bool("print_with_variable_declarations", true)
	return config, err
}
