package smt2

// Source: src/cmd_context/tactic_cmds.cpp sexpr2tactic and mk_using_params;
// src/parsers/smt2/smt2parser.cpp command argument parsing.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// parseTactic returns one counted reference, including on the one-child
// combinator paths where the native factory returns its child unchanged.
func (p *commandParser) parseTactic() (result tactic.Tactic, err error) {
	if p.token == SymbolToken {
		name, e := p.symbol()
		if e != nil {
			return nil, e
		}
		result, err = p.context.NewBuiltinTacticWithOutput(name, p.out)
		if err != nil {
			return nil, err
		}
		if err = result.IncRef(); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err = p.expect(LeftParenToken); err != nil {
		return nil, fmt.Errorf("invalid tactic, symbol or combinator expected")
	}
	name, err := p.symbol()
	if err != nil {
		return nil, err
	}
	var children []tactic.Tactic
	defer func() {
		for _, child := range children {
			err = errors.Join(err, child.DecRef())
		}
	}()
	child := func() error {
		value, e := p.parseTactic()
		if e != nil {
			return e
		}
		children = append(children, value)
		return nil
	}
	switch name {
	case "and-then", "then", "or-else":
		for p.token != RightParenToken && p.token != EOFToken {
			if err = child(); err != nil {
				return nil, err
			}
		}
		if len(children) == 0 {
			return nil, fmt.Errorf("invalid %s combinator, at least one argument expected", name)
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		if name == "or-else" {
			if len(children) == 1 {
				result = children[0]
			} else {
				result, err = tactic.NewOrElseTactic(children...)
			}
		} else {
			result, err = tactic.NewAndThenTactic(children...)
		}
	case "using-params", "with", "!":
		if err = child(); err != nil {
			return nil, err
		}
		if p.token == RightParenToken {
			if err = p.expect(RightParenToken); err != nil {
				return nil, err
			}
			result = children[0]
			break
		}
		globals, e := p.context.GlobalParameters()
		if e != nil {
			return nil, e
		}
		c := globals.Context
		settings, e := c.NewValue()
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, c.Del(&settings)) }()
		m, e := p.astManager()
		if e != nil {
			return nil, e
		}
		descriptions, e := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, descriptions.Close()) }()
		if e = children[0].CollectParamDescrs(descriptions); e != nil {
			return nil, e
		}
		for p.token != RightParenToken && p.token != EOFToken {
			if p.token != KeywordToken {
				return nil, fmt.Errorf("invalid using-params combinator, keyword expected")
			}
			key := strings.TrimPrefix(p.scanner.ID(), ":")
			key = uparams.NormParamName(&key)
			if err = p.advance(); err != nil {
				return nil, err
			}
			symbol, e := m.Symbols.Intern(key)
			if e != nil {
				return nil, e
			}
			kind, e := descriptions.KindInModule(&symbol)
			if e != nil {
				return nil, e
			}
			key, e = m.Symbols.String(symbol)
			if e != nil {
				return nil, e
			}
			if kind == uparams.ParamInvalid {
				return nil, fmt.Errorf("invalid using-params combinator, unknown parameter %s", key)
			}
			if err = p.tacticParameter(c, &settings, key, kind); err != nil {
				return nil, err
			}
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		result, err = tactic.NewUsingParamsTactic(c, children[0], &settings)
	case "repeat", "try-for", "fail-if-branching":
		if err = child(); err != nil {
			return nil, err
		}
		n := ^uint32(0)
		if name == "fail-if-branching" {
			n = 1
		}
		if p.token != RightParenToken || name == "try-for" {
			if n, err = p.unsigned(); err != nil {
				return nil, err
			}
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		switch name {
		case "repeat":
			result, err = tactic.NewRepeatTactic(children[0], n)
		case "try-for":
			result, err = tactic.NewTryForTactic(children[0], n)
		default:
			result, err = tactic.NewFailIfBranchingTactic(children[0], n)
		}
	case "skip-if-failed":
		if err = child(); err != nil {
			return nil, err
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		result, err = tactic.NewSkipIfFailedTactic(children[0])
	default:
		return nil, fmt.Errorf("invalid tactic combinator %s", name)
	}
	if err != nil {
		return nil, err
	}
	if err = result.IncRef(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *commandParser) tacticParameter(c *uparams.ParamsContext, settings *uparams.ParamsRef, key string, kind uparams.ParamKind) (err error) {
	switch kind {
	case uparams.ParamBool:
		if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
			return fmt.Errorf("invalid parameter value, true or false expected")
		}
		if err = c.SetBoolName(settings, &key, p.scanner.ID() == "true"); err != nil {
			return err
		}
	case uparams.ParamUInt:
		if p.token != IntegerToken && p.token != DecimalToken {
			return fmt.Errorf("invalid parameter value, unsigned integer expected")
		}
		n := p.scanner.Number()
		if !n.IsInt() || n.Sign() < 0 || !n.Num().IsUint64() || n.Num().Uint64() > uint64(^uint32(0)) {
			return fmt.Errorf("invalid parameter value, unsigned integer expected")
		}
		if err = c.SetUIntName(settings, &key, uint32(n.Num().Uint64())); err != nil {
			return err
		}
	case uparams.ParamDouble, uparams.ParamNumeral:
		if p.token != IntegerToken && p.token != DecimalToken {
			return fmt.Errorf("invalid parameter value, numeral expected")
		}
		n := p.scanner.Number()
		if kind == uparams.ParamDouble {
			value, _ := n.Float64()
			if err = c.SetDoubleName(settings, &key, value); err != nil {
				return err
			}
		} else {
			r, e := c.Numerals().NewValue()
			if e != nil {
				return e
			}
			defer func() { err = errors.Join(err, c.Numerals().Del(&r)) }()
			if err = c.Numerals().SetString(&r, n.RatString()); err != nil {
				return err
			}
			if err = c.SetRatName(settings, &key, &r); err != nil {
				return err
			}
		}
	case uparams.ParamSymbol:
		if p.token != SymbolToken {
			return fmt.Errorf("invalid parameter value, symbol expected")
		}
		sym, e := c.SymbolsField.Intern(p.scanner.ID())
		if e != nil {
			return e
		}
		if err = c.SetSymbolName(settings, &key, sym); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid using-params combinator, unsupported parameter kind")
	}
	return p.advance()
}

func (p *commandParser) checkSatUsing(execute bool) (err error) {
	if p.token == RightParenToken {
		return fmt.Errorf("check-sat-using needs a tactic argument")
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
		key := strings.TrimPrefix(p.scanner.ID(), ":")
		key = uparams.NormParamName(&key)
		if key != "timeout" {
			return fmt.Errorf("unsupported check-sat-using option %s", key)
		}
		if err = p.advance(); err != nil {
			return err
		}
		if err = p.tacticParameter(c, &options, key, uparams.ParamUInt); err != nil {
			return err
		}
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	if !execute {
		return nil
	}
	status, err := p.context.CheckSatUsing(selected, &options)
	if err != nil {
		var failure cmd_context.TacticCommandFailure
		if errors.As(err, &failure) {
			_, err = fmt.Fprintf(p.out, "(error \"tactic failed: %s\")\n", failure.Error())
			return err
		}
		return err
	}
	text := "unknown"
	if status == lbool.True {
		text = "sat"
	} else if status == lbool.False {
		text = "unsat"
	}
	if _, err = fmt.Fprintln(p.out, text); err != nil {
		return err
	}
	return p.context.ValidateInfoStatus(status)
}
