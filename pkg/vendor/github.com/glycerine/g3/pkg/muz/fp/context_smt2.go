package fp

// Source: muz/base/dl_context.cpp display_smt2 and fixedpoint-local print
// parameters and pending closed input formulas.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (e *PropositionalEngine) contextSMT2Config(queries []ast.ASTHandle, local *uparams.ParamsRef) (config base.ContextSMT2Config, resultErr error) {
	config = base.DefaultContextSMT2Config()
	config.Queries = queries
	if e.params != nil && e.globals != nil {
		parameters, err := e.fixedpointParamView()
		if err != nil {
			return config, err
		}
		defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
		if local != nil {
			parameters.local = local
		}
		config.FixedpointExtensions, err = parameters.Bool("print_fixedpoint_extensions", true)
		if err != nil {
			return config, err
		}
		config.LowLevel, err = parameters.Bool("print_low_level_smt2", false)
		if err != nil {
			return config, err
		}
		config.DeclareVariables, err = parameters.Bool("print_with_variable_declarations", true)
		if err != nil {
			return config, err
		}
	}
	for _, ref := range e.assertions {
		h, err := e.manager.RefGet(ref)
		if err != nil {
			return config, err
		}
		config.Background = append(config.Background, h)
	}
	if len(e.assertions) == 0 {
		h, err := e.manager.RefGet(e.background)
		if err != nil {
			return config, err
		}
		if !e.manager.IsTrue(h) {
			config.Background = append(config.Background, h)
		}
	}
	return config, nil
}

// WriteContextSMT2 emits source print profiles without changing query options
// or invalidating a published result. Local parameters, when given, override
// engine/module print settings for this invocation only.
func (e *PropositionalEngine) WriteContextSMT2(out io.Writer, queries []ast.ASTHandle, local *uparams.ParamsRef) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	config, err := e.contextSMT2Config(queries, local)
	if err != nil {
		return err
	}
	rules, pending, names, err := e.contextFormulaRules()
	if err != nil {
		return err
	}
	if rules != e.set {
		a := &contextFormulaAttempt{manager: e.manager, sets: []*base.RuleSet{rules}}
		defer func() { resultErr = a.finish(resultErr) }()
	}
	config.PendingRules = pending
	config.PendingNameSymbols = names
	return rules.WriteContextSMT2(out, config)
}
