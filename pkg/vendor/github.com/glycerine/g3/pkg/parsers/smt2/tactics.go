package smt2

// Source: src/cmd_context/tactic_cmds.cpp sexpr2tactic and mk_using_params;
// src/parsers/smt2/smt2parser.cpp command argument parsing.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
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
		if !cmd_context.IsBuiltinTactic(name) {
			if body, found := p.context.UserTacticBody(name); found {
				return p.parseDeclaredTactic(name, body)
			}
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
	case "par", "par-or", "par-then":
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
		if len(children) == 1 {
			result = children[0]
		} else if name == "par-then" {
			result, err = tactic.NewParAndThenTactic(children...)
		} else {
			result, err = tactic.NewParTactic(children...)
		}
	case "if", "ite", "cond", "when", "fail-if":
		probe, e := p.parseProbe()
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, probe.DecRef()) }()
		if name == "fail-if" {
			if err = p.expect(RightParenToken); err != nil {
				return nil, err
			}
			result, err = tactic.NewFailIfTactic(probe)
		} else {
			if err = child(); err != nil {
				return nil, err
			}
			if name == "when" {
				if err = p.expect(RightParenToken); err != nil {
					return nil, err
				}
				result, err = tactic.NewWhenTactic(probe, children[0])
			} else {
				if err = child(); err != nil {
					return nil, err
				}
				if err = p.expect(RightParenToken); err != nil {
					return nil, err
				}
				result, err = tactic.NewCondTactic(probe, children[0], children[1])
			}
		}
	case "if-no-proofs", "if-no-models", "if-no-unsat-cores":
		if err = child(); err != nil {
			return nil, err
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		switch name {
		case "if-no-proofs":
			result, err = tactic.NewIfNoProofsTactic(children[0])
		case "if-no-models":
			result, err = tactic.NewIfNoModelsTactic(children[0])
		default:
			result, err = tactic.NewIfNoUnsatCoresTactic(children[0])
		}
	case "echo":
		var steps []tactic.Tactic
		defer func() {
			for _, step := range steps {
				err = errors.Join(err, step.DecRef())
			}
		}()
		for p.token != RightParenToken && p.token != EOFToken {
			last := p.scanner
			var step tactic.Tactic
			if p.token == StringToken {
				message := last.String()
				if err = p.advance(); err != nil {
					return nil, err
				}
				step, err = cmd_context.NewEchoTactic(p.context, message, p.token == RightParenToken)
			} else {
				probe, e := p.parseProbe()
				if e != nil {
					return nil, e
				}
				step, err = cmd_context.NewProbeValueTactic(p.context, nil, probe, p.token == RightParenToken)
				err = errors.Join(err, probe.DecRef())
			}
			if err != nil {
				return nil, err
			}
			if err = step.IncRef(); err != nil {
				return nil, err
			}
			steps = append(steps, step)
		}
		if len(steps) == 0 {
			return nil, fmt.Errorf("invalid echo tactic, must have at least one argument")
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		if len(steps) == 1 {
			result = steps[0]
		} else {
			result, err = tactic.NewAndThenTactic(steps...)
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

func (p *commandParser) parseProbe() (result tactic.ProbeEvaluator, err error) {
	manager, err := p.astManager()
	if err != nil {
		return nil, err
	}
	if p.token == IntegerToken || p.token == DecimalToken {
		value := p.scanner.Number()
		if !value.IsInt() || !value.Num().IsInt64() || value.Num().Int64() < -2147483648 || value.Num().Int64() > 2147483647 {
			return nil, fmt.Errorf("invalid probe, constant is too big to fit in a fixed size integer")
		}
		if err = p.advance(); err != nil {
			return nil, err
		}
		result, err = tactic.NewProbe(manager.Budget(), tactic.ProbeConstant, float64(value.Num().Int64()))
	} else if p.token == SymbolToken {
		name, e := p.symbol()
		if e != nil {
			return nil, e
		}
		result, err = p.sourceBuiltinProbe(name, manager)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("invalid probe, unknown builtin probe %s", name)
		}
	} else if p.token == LeftParenToken {
		if err = p.advance(); err != nil {
			return nil, err
		}
		name, e := p.symbol()
		if e != nil {
			return nil, e
		}
		var arguments []tactic.ProbeEvaluator
		defer func() {
			for _, argument := range arguments {
				err = errors.Join(err, argument.DecRef())
			}
		}()
		for p.token != RightParenToken && p.token != EOFToken {
			argument, e := p.parseProbe()
			if e != nil {
				return nil, e
			}
			arguments = append(arguments, argument)
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, err
		}
		if name == "not" {
			if len(arguments) != 1 {
				return nil, fmt.Errorf("invalid probe expression, one argument expected")
			}
			result, err = tactic.NewNotProbe(manager.Budget(), arguments[0])
		} else {
			kind, found := sourceProbeOperator(name)
			if !found {
				return nil, fmt.Errorf("invalid probe, unknown probe expression %s", name)
			}
			if len(arguments) == 0 {
				return nil, fmt.Errorf("invalid probe, at least one argument expected")
			}
			if kind >= tactic.ProbeEqual && kind <= tactic.ProbeGreater {
				if len(arguments) != 2 {
					return nil, fmt.Errorf("invalid probe expression, two arguments expected")
				}
				result, err = tactic.NewBinaryProbe(manager.Budget(), kind, arguments[0], arguments[1])
			} else if len(arguments) == 1 {
				result = arguments[0]
			} else {
				result = arguments[0]
				for _, argument := range arguments[1:] {
					next, e := tactic.NewBinaryProbe(manager.Budget(), kind, result, argument)
					if e != nil {
						return nil, e
					}
					result = next
				}
			}
		}
	} else {
		return nil, fmt.Errorf("invalid probe, unexpected input")
	}
	if err != nil {
		return nil, err
	}
	if err = result.IncRef(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *commandParser) sourceBuiltinProbe(name string, manager *ast.ASTManager) (tactic.ProbeEvaluator, error) {
	kinds := map[string]tactic.ProbeKind{
		"memory": tactic.ProbeMemory, "depth": tactic.ProbeDepth, "size": tactic.ProbeSize,
		"num-exprs": tactic.ProbeNumExprs, "num-consts": tactic.ProbeNumConsts,
		"num-bool-consts": tactic.ProbeNumBoolConsts, "num-arith-consts": tactic.ProbeNumArithmeticConsts,
		"num-bv-consts": tactic.ProbeNumBitVectorConsts, "produce-proofs": tactic.ProbeProduceProofs,
		"produce-model": tactic.ProbeProduceModels, "produce-unsat-cores": tactic.ProbeProduceUnsatCores,
		"has-quantifiers": tactic.ProbeHasQuantifier, "has-patterns": tactic.ProbeHasPattern,
		"is-propositional": tactic.ProbeIsPropositional, "is-qfbv": tactic.ProbeIsQFBitVector,
		"is-qfaufbv": tactic.ProbeIsQFArrayUFBitVector, "is-qfufbv": tactic.ProbeIsQFUFBitVector,
	}
	kind, found := kinds[name]
	if found {
		return tactic.NewProbe(manager.Budget(), kind, 0)
	}
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return nil, err
	}
	context := globals.Context
	switch name {
	case "ackr-bound-probe":
		return ackermannization.NewBoundProbe(manager.Budget())
	case "is-qflia", "is-qfauflia", "is-qflra", "is-qflira", "is-ilp", "is-qfnia", "is-qfnra", "is-nia", "is-nra", "is-nira", "is-lia", "is-lra", "is-lira", "is-qfufnra":
		fragments := map[string]arith.ArithmeticFragment{
			"is-qflia": arith.FragmentQFLIA, "is-qfauflia": arith.FragmentQFAUFLIA,
			"is-qflra": arith.FragmentQFLRA, "is-qflira": arith.FragmentQFLIRA,
			"is-ilp": arith.FragmentILP, "is-qfnia": arith.FragmentQFNIA,
			"is-qfnra": arith.FragmentQFNRA, "is-nia": arith.FragmentNIA,
			"is-nra": arith.FragmentNRA, "is-nira": arith.FragmentNIRA,
			"is-lia": arith.FragmentLIA, "is-lra": arith.FragmentLRA,
			"is-lira": arith.FragmentLIRA, "is-qfufnra": arith.FragmentQFUFNRA,
		}
		return arith.NewArithmeticFragmentProbe(manager.Budget(), context, globals, fragments[name])
	case "is-unbounded":
		return arith.NewIsUnboundedProbe(manager.Budget(), context, globals)
	case "is-pb":
		return arith.NewIsPBProbe(manager.Budget(), context, globals)
	case "is-quasi-pb":
		return smtlogics.NewIsQuasiPBProbe(manager, context, globals)
	case "arith-max-deg":
		return arith.NewArithmeticMaximumDegreeProbe(manager.Budget())
	case "arith-avg-deg":
		return arith.NewArithmeticAverageDegreeProbe(manager.Budget())
	case "arith-max-bw":
		return arith.NewArithmeticMaximumBitWidthProbe(manager.Budget())
	case "arith-avg-bw":
		return arith.NewArithmeticAverageBitWidthProbe(manager.Budget())
	case "is-qfbv-eq":
		return bv.NewIsQFBVEqualityProbe(manager.Budget(), context, globals)
	case "is-qffp", "is-qffpbv":
		return smtlogics.NewIsQFFPProbe(manager.Budget())
	case "is-qffplra":
		return smtlogics.NewIsQFFPLRAProbe(manager.Budget())
	default:
		return nil, nil
	}
}

func sourceProbeOperator(name string) (tactic.ProbeKind, bool) {
	kinds := map[string]tactic.ProbeKind{
		"=": tactic.ProbeEqual, "<=": tactic.ProbeLessEqual, ">=": tactic.ProbeGreaterEqual,
		"<": tactic.ProbeLess, ">": tactic.ProbeGreater, "and": tactic.ProbeAnd,
		"or": tactic.ProbeOr, "=>": tactic.ProbeImplies, "implies": tactic.ProbeImplies,
		"*": tactic.ProbeMultiply, "+": tactic.ProbeAdd, "-": tactic.ProbeSubtract, "/": tactic.ProbeDivide,
	}
	kind, found := kinds[name]
	return kind, found
}

func (p *commandParser) parseDeclaredTactic(name, body string) (result tactic.Tactic, err error) {
	for _, active := range p.tacticExpansion {
		if active == name {
			return nil, fmt.Errorf("recursive user tactic %s", name)
		}
	}
	compliant, err := p.context.Compliant()
	if err != nil {
		return nil, err
	}
	subscanner, err := NewScanner(strings.NewReader(body+" "), compliant)
	if err != nil {
		return nil, err
	}
	previousScanner, previousToken, previousEnd := p.scanner, p.token, p.cacheEnd
	p.scanner, p.token, p.cacheEnd = subscanner, NullToken, 0
	p.tacticExpansion = append(p.tacticExpansion, name)
	defer func() {
		p.tacticExpansion = p.tacticExpansion[:len(p.tacticExpansion)-1]
		p.scanner, p.token, p.cacheEnd = previousScanner, previousToken, previousEnd
		if err != nil && result != nil {
			err = errors.Join(err, result.DecRef())
			result = nil
		}
	}()
	if err = p.advance(); err != nil {
		return nil, err
	}
	result, err = p.parseTactic()
	if err != nil {
		return nil, err
	}
	if p.token != EOFToken {
		err = errors.Join(fmt.Errorf("invalid user tactic %s: trailing input", name), result.DecRef())
		result = nil
		return nil, err
	}
	return result, nil
}

func (p *commandParser) parseTacticDeclaration() (selected tactic.Tactic, body string, err error) {
	if p.token == SymbolToken {
		body = p.scanner.ID()
		selected, err = p.parseTactic()
		return selected, body, err
	}
	if p.token != LeftParenToken {
		return nil, "", fmt.Errorf("invalid tactic, symbol or combinator expected")
	}
	p.scanner.StartCaching()
	defer func() { p.scanner.StopCaching(); p.scanner.ResetCache() }()
	selected, err = p.parseTactic()
	if err != nil {
		return nil, "", err
	}
	body, err = p.scanner.CachedString(0, p.cacheEnd)
	if err != nil {
		return nil, "", errors.Join(err, selected.DecRef())
	}
	return selected, "(" + body, nil
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
	case uparams.ParamString:
		if p.token != StringToken {
			return fmt.Errorf("invalid parameter value, string expected")
		}
		value := p.scanner.String()
		if err = c.SetStringName(settings, &key, &value); err != nil {
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
	printStatistics, printUnsatCore, printProof, printModel := false, false, false, false
	for p.token == KeywordToken {
		key, e := p.parametricKeyword()
		if e != nil {
			return e
		}
		kind := uparams.ParamKind(0)
		switch key {
		case "timeout", "max_memory":
			kind = uparams.ParamUInt
		case "print_statistics":
			kind = uparams.ParamBool
		case "print_unsat_core":
			kind = uparams.ParamBool
		case "print_proof":
			kind = uparams.ParamBool
		case "print_model":
			kind = uparams.ParamBool
		default:
			return fmt.Errorf("invalid keyword argument")
		}
		if err = p.advance(); err != nil {
			return err
		}
		if kind == uparams.ParamUInt && p.token != IntegerToken {
			return fmt.Errorf("invalid command argument, unsigned integer expected")
		}
		if err = p.tacticParameter(c, &options, key, kind); err != nil {
			return err
		}
		if kind == uparams.ParamBool {
			value, e := c.GetBoolName(&options, &key, false)
			if e != nil {
				return e
			}
			switch key {
			case "print_statistics":
				printStatistics = value
			case "print_unsat_core":
				printUnsatCore = value
			case "print_proof":
				printProof = value
			case "print_model":
				printModel = value
			}
		}
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	if !execute {
		return nil
	}
	status, err := p.context.CheckSatUsing(selected, &options)
	tacticFailure := false
	if err != nil {
		var failure cmd_context.TacticCommandFailure
		if errors.As(err, &failure) {
			_, err = fmt.Fprintf(p.out, "(error \"tactic failed: %s\")\n", failure.Error())
			if err != nil {
				return err
			}
			status = lbool.Undef
			tacticFailure = true
		} else {
			return err
		}
	}
	if !tacticFailure {
		text := "unknown"
		if status == lbool.True {
			text = "sat"
		} else if status == lbool.False {
			text = "unsat"
		}
		if _, err = fmt.Fprintln(p.out, text); err != nil {
			return err
		}
	}
	if err = p.context.ValidateInfoStatus(status); err != nil {
		return err
	}
	produceModels, produceProofs, produceCores, err := p.context.ResultProductionEnabled()
	if err != nil {
		return err
	}
	if printUnsatCore && produceCores && status == lbool.False {
		core, e := p.context.UnsatCore()
		if e != nil {
			return e
		}
		if _, err = fmt.Fprint(p.out, "(unsat-core"); err != nil {
			return err
		}
		for _, expression := range core {
			if _, err = fmt.Fprint(p.out, " "); err != nil {
				return err
			}
			if err = p.context.WriteExpression(p.out, expression); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintln(p.out, ")"); err != nil {
			return err
		}
	}
	if printModel && produceModels {
		if err = p.context.WriteModel(p.out); err != nil && err.Error() != "model is not available" {
			return err
		}
	}
	if printProof && produceProofs && status == lbool.False {
		if err = p.context.WriteProof(p.out); err != nil && err.Error() != "proof is not available" {
			return err
		}
	}
	if printStatistics {
		return p.context.WriteStatistics(p.out)
	}
	return nil
}
