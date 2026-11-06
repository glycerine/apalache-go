package smt2

// Source: src/cmd_context/tactic_cmds.cpp:249-347, apply_tactic_cmd.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/cmd_context"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var applyTacticParameterKinds = map[string]uparams.ParamKind{
	"print":                 uparams.ParamBool,
	"print_proof":           uparams.ParamBool,
	"print_model_converter": uparams.ParamBool,
	"print_benchmark":       uparams.ParamBool,
	"print_dependencies":    uparams.ParamBool,
	"print_statistics":      uparams.ParamBool,
	"timeout":               uparams.ParamUInt,
	"rlimit":                uparams.ParamUInt,
	"max_memory":            uparams.ParamUInt,
}

func (p *commandParser) applyTactic(execute bool) (err error) {
	if p.token == RightParenToken {
		return fmt.Errorf("apply needs a tactic argument")
	}
	selected, err := p.parseTactic()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, selected.DecRef()) }()
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return err
	}
	c := globals.Context
	options, err := c.NewValue()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.Del(&options)) }()
	for p.token == KeywordToken {
		key, e := p.parametricKeyword()
		if e != nil {
			return e
		}
		kind, found := applyTacticParameterKinds[key]
		if !found {
			return fmt.Errorf("invalid keyword argument")
		}
		if err = p.advance(); err != nil {
			return err
		}
		if err = p.tacticParameter(c, &options, key, kind); err != nil {
			return err
		}
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	if !execute {
		return nil
	}
	if err = p.context.ApplyTactic(p.out, selected, &options); err != nil {
		var failure cmd_context.TacticCommandFailure
		if errors.As(err, &failure) {
			_, err = fmt.Fprintf(p.out, "(error \"tactic failed: %s\")\n", failure.Error())
		}
	}
	return err
}
