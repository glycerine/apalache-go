package smt2

// Source: src/parsers/smt2/smt2parser.cpp command dispatch and retained
// declaration/assertion/check commands; src/cmd_context/basic_cmds.cpp.

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type commandParser struct {
	context            *cmd_context.Context
	scanner            *Scanner
	token              Token
	cacheEnd           int
	out                io.Writer
	manager            *ast.ASTManager
	refs               []ast.ASTRef
	env                smt2LocalEnv
	locals             localExprStack
	bindings           uint32
	shifter            *rewriter.VarShifter
	sortParams         map[string]ast.ASTHandle
	datatypeHeaders    map[string]uint32
	ignoreUserPatterns bool
	ignoreBadPatterns  bool
	ignoreChecks       bool
	softWeight         big.Rat
	softID             string
}

// ParseCommands executes the retained SMT-LIB command grammar against Context.
// Unsupported commands fail explicitly. Command-local AST references are
// released after execution; the context owns declarations and assertions.
func ParseCommands(context *cmd_context.Context, input io.Reader, out io.Writer) (result error) {
	return parseCommands(context, input, out, false)
}

// ParseAssertions parses a solver-loading SMT-LIB stream. Check commands are
// syntax-checked but not executed or printed; callers consume the context's
// final assertion set only, matching api_solver.cpp solver_from_stream.
func ParseAssertions(context *cmd_context.Context, input io.Reader) error {
	return parseCommands(context, input, io.Discard, true)
}

func parseCommands(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool) (result error) {
	if err := context.SetRegularOutput(out); err != nil {
		return err
	}
	// Source gparams is process-wide. During this single-goroutine command
	// session, lazy AST algebraic owners and printers must see this context's
	// parameter service. Restore the enclosing session after parser cleanup.
	globals, err := context.GlobalParameters()
	if err != nil {
		return err
	}
	previous, err := uparams.BindProcessGlobalParamsService(globals)
	if err != nil {
		return err
	}
	defer func() {
		_, err := uparams.BindProcessGlobalParamsService(previous)
		result = errors.Join(result, err)
	}()
	compliant, err := context.Compliant()
	if err != nil {
		return err
	}
	scanner, err := NewScanner(input, compliant)
	if err != nil {
		return err
	}
	p := &commandParser{context: context, scanner: scanner, out: out, ignoreChecks: ignoreChecks}
	p.softWeight.SetInt64(1)
	defer func() {
		result = errors.Join(result, p.clearCommand())
		if p.shifter != nil {
			result = errors.Join(result, p.shifter.Close())
		}
	}()
	if err = p.configurePatternOptions(); err != nil {
		return err
	}
	if err = p.advance(); err != nil {
		return err
	}
	for p.token != EOFToken {
		exit, err := p.command()
		if err != nil {
			return &ScannerError{err.Error(), scanner.Line(), scanner.Position()}
		}
		if err = p.clearCommand(); err != nil {
			return err
		}
		if exit {
			return nil
		}
	}
	return nil
}

func (p *commandParser) advance() error {
	p.cacheEnd = p.scanner.CacheSize()
	var err error
	p.token, err = p.scanner.Scan()
	return err
}
func (p *commandParser) expect(token Token) error {
	if p.token != token {
		return fmt.Errorf("unexpected input, expected token %d", token)
	}
	return p.advance()
}

func (p *commandParser) requireEnd() error {
	if p.token != RightParenToken {
		return fmt.Errorf("invalid command, ')' expected")
	}
	return nil
}
func (p *commandParser) symbol() (string, error) {
	if p.token != SymbolToken {
		return "", fmt.Errorf("symbol expected")
	}
	value := p.scanner.ID()
	return value, p.advance()
}
func (p *commandParser) unsigned() (uint32, error) {
	if p.token != IntegerToken {
		return 0, fmt.Errorf("unsigned numeral expected")
	}
	n := p.scanner.Number()
	if !n.IsInt() || n.Sign() < 0 || !n.Num().IsUint64() || n.Num().Uint64() > uint64(^uint32(0)) {
		return 0, fmt.Errorf("unsigned numeral is out of range")
	}
	value := uint32(n.Num().Uint64())
	return value, p.advance()
}
func (p *commandParser) rational() (*big.Rat, error) {
	if p.token != IntegerToken && p.token != DecimalToken {
		return nil, fmt.Errorf("rational numeral expected")
	}
	value := p.scanner.Number()
	return value, p.advance()
}
func (p *commandParser) astManager() (*ast.ASTManager, error) {
	if p.manager != nil {
		return p.manager, nil
	}
	m, err := p.context.Manager()
	if err != nil {
		return nil, err
	}
	p.manager = m
	p.shifter, err = rewriter.NewVarShifter(m)
	return m, err
}
func (p *commandParser) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	return value, nil
}
func (p *commandParser) clearCommand() (result error) {
	if p.manager != nil {
		result = p.locals.reset(p.manager)
		for i := len(p.refs) - 1; i >= 0; i-- {
			result = errors.Join(result, p.manager.DelRef(&p.refs[i]))
		}
	}
	p.refs = nil
	p.env.reset()
	p.bindings = 0
	return result
}

func (p *commandParser) command() (bool, error) {
	commandLine, commandPosition := p.scanner.Line(), p.scanner.Position()
	if err := p.expect(LeftParenToken); err != nil {
		return false, err
	}
	name, err := p.symbol()
	if err != nil {
		return false, err
	}
	success := true
	switch name {
	case "set-logic":
		logic, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.SetLogic(logic); err != nil {
			return false, err
		}
	case "set-option":
		if p.token != KeywordToken {
			return false, fmt.Errorf("option keyword expected")
		}
		key := strings.TrimPrefix(p.scanner.ID(), ":")
		if err := p.advance(); err != nil {
			return false, err
		}
		if p.token == StringToken {
			value := p.scanner.String()
			if err = p.advance(); err != nil {
				return false, err
			}
			if err = p.requireEnd(); err != nil {
				return false, err
			}
			if err = p.context.SetStringOption(key, value); err != nil {
				return false, err
			}
			break
		}
		if p.token == IntegerToken {
			value, err := p.unsigned()
			if err != nil {
				return false, err
			}
			if err = p.requireEnd(); err != nil {
				return false, err
			}
			if err = p.context.SetUIntOption(key, value); err != nil {
				return false, err
			}
			break
		}
		value, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if value == "true" || value == "false" {
			err = p.context.SetBoolOption(key, value == "true")
		} else {
			err = p.context.SetSymbolOption(key, value)
		}
		if err != nil {
			return false, err
		}
		if err = p.configurePatternOptions(); err != nil {
			return false, err
		}
	case "set-info":
		if p.token != KeywordToken {
			return false, fmt.Errorf("information keyword expected")
		}
		key := p.scanner.ID()
		if err := p.advance(); err != nil {
			return false, err
		}
		if key == ":status" && p.token == SymbolToken {
			value, e := p.symbol()
			if e != nil {
				return false, e
			}
			if err = p.context.SetInfoStatus(value); err != nil {
				return false, err
			}
		} else {
			// Uninterpreted source metadata is accepted and discarded.
			if err := p.skipSExpr(); err != nil {
				return false, err
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
	case "declare-datatype", "declare-datatypes":
		if err = p.declareDatatypes(name == "declare-datatype"); err != nil {
			return false, err
		}
	case "declare-sort":
		name, err := p.symbol()
		if err != nil {
			return false, err
		}
		var arity uint32
		if p.token != RightParenToken {
			arity, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if _, err = p.context.DeclareSortConstructor(name, arity); err != nil {
			return false, err
		}
	case "declare-map":
		mapName, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.expect(LeftParenToken); err != nil {
			return false, err
		}
		var indices []ast.ASTHandle
		for p.token != RightParenToken {
			sort, e := p.sort()
			if e != nil {
				return false, e
			}
			indices = append(indices, sort)
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		function, err := p.functionDeclarationReference()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if _, err = p.context.DeclareMap(mapName, indices, function); err != nil {
			return false, err
		}
	case "define-sort":
		name, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.expect(LeftParenToken); err != nil {
			return false, err
		}
		var parameterNames []string
		for p.token != RightParenToken {
			parameter, e := p.symbol()
			if e != nil {
				return false, e
			}
			parameterNames = append(parameterNames, parameter)
		}
		if err = p.expect(RightParenToken); err != nil {
			return false, err
		}
		formals, byName, err := p.datatypeFormalParameters(parameterNames)
		if err != nil {
			return false, err
		}
		previous := p.sortParams
		p.sortParams = byName
		body, bodyErr := p.sort()
		p.sortParams = previous
		if bodyErr != nil {
			return false, bodyErr
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.DefineSort(name, formals, body); err != nil {
			return false, err
		}
	case "declare-fun", "declare-const":
		function, err := p.symbol()
		if err != nil {
			return false, err
		}
		var domain []ast.ASTHandle
		if name == "declare-fun" {
			if err = p.expect(LeftParenToken); err != nil {
				return false, err
			}
			for p.token != RightParenToken {
				sort, err := p.sort()
				if err != nil {
					return false, err
				}
				domain = append(domain, sort)
			}
			if err = p.advance(); err != nil {
				return false, err
			}
		}
		rangeSort, err := p.sort()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if _, err = p.context.DeclareFunction(function, domain, rangeSort); err != nil {
			return false, err
		}
	case "declare-rel":
		relation, err := p.symbol()
		if err != nil {
			return false, err
		}
		if err = p.expect(LeftParenToken); err != nil {
			return false, err
		}
		var domain []ast.ASTHandle
		for p.token != RightParenToken {
			sort, x := p.sort()
			if x != nil {
				return false, x
			}
			domain = append(domain, sort)
		}
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if _, err = p.context.DeclareRelation(relation, domain); err != nil {
			return false, err
		}
	case "declare-var":
		variable, err := p.symbol()
		if err != nil {
			return false, err
		}
		sort, err := p.sort()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.DeclareFixedpointVariable(variable, sort); err != nil {
			return false, err
		}
	case "define-fun", "define-const":
		if err := p.defineFunction(name == "define-fun"); err != nil {
			return false, err
		}
	case "define-fun-rec", "define-funs-rec":
		if err := p.defineRecursiveFunctions(name == "define-funs-rec"); err != nil {
			return false, err
		}
	case "assert":
		expression, named, displayText, err := p.assertionExpression()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.AssertWithText(expression, named, displayText); err != nil {
			return false, err
		}
	case "rule":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		name := ""
		bound := ^uint32(0)
		if p.token != RightParenToken {
			name, err = p.symbol()
			if err != nil {
				return false, err
			}
		}
		if p.token != RightParenToken {
			bound, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.AddFixedpointRuleWithBound(expression, name, bound); err != nil {
			return false, err
		}
	case "fixedpoint-push", "fixedpoint-pop":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if name == "fixedpoint-push" {
			err = p.context.PushFixedpoint()
		} else {
			err = p.context.PopFixedpoint()
		}
		if err != nil {
			return false, err
		}
	case "assert-soft":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		weight := new(big.Rat).Set(&p.softWeight)
		id := p.softID
		for p.token != RightParenToken {
			if p.token != KeywordToken {
				return false, fmt.Errorf("assert-soft option keyword expected")
			}
			key := p.scanner.ID()
			if err = p.advance(); err != nil {
				return false, err
			}
			switch key {
			case ":weight":
				weight, err = p.rational()
				if err == nil {
					p.softWeight.Set(weight)
				}
			case ":id":
				id, err = p.symbol()
				if err == nil {
					p.softID = id
				}
			default:
				return false, fmt.Errorf("unsupported assert-soft option %s", key)
			}
			if err != nil {
				return false, err
			}
		}
		if err = p.context.AddSoftConstraint(expression, weight, id); err != nil {
			return false, err
		}
	case "maximize", "minimize":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.AddObjective(expression, name == "maximize"); err != nil {
			return false, err
		}
	case "push", "pop":
		n := uint32(1)
		if p.token != RightParenToken {
			n, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if name == "push" {
			err = p.context.Push(n)
		} else {
			err = p.context.Pop(n)
		}
		if err != nil {
			return false, err
		}
		// Source parse_push/parse_pop mutates the context before checking
		// the closing parenthesis; a trailing syntax error keeps that prefix.
		if err = p.requireEnd(); err != nil {
			return false, err
		}
	case "check-sat-using":
		if err := p.checkSatUsing(!p.ignoreChecks); err != nil {
			return false, err
		}
		success = false
	case "check-sat", "check-sat-assuming":
		var assumptions []ast.ASTHandle
		if name == "check-sat-assuming" {
			assumptions, err = p.expressions()
			if err != nil {
				return false, err
			}
		}
		if p.token != RightParenToken {
			return false, fmt.Errorf("unexpected check-sat argument")
		}
		if !p.ignoreChecks {
			status, err := p.context.CheckSat(assumptions)
			if err != nil {
				return false, err
			}
			text := "unknown"
			if status == lbool.True {
				text = "sat"
			}
			if status == lbool.False {
				text = "unsat"
			}
			if _, err = fmt.Fprintln(p.out, text); err != nil {
				return false, err
			}
			if err = p.context.ValidateInfoStatus(status); err != nil {
				return false, err
			}
		}
		success = false
	case "query":
		declaration, err := p.functionDeclarationReference()
		if err != nil {
			return false, err
		}
		printAnswer, printStatistics, printCertificate := false, false, false
		for p.token != RightParenToken {
			if p.token != KeywordToken {
				return false, fmt.Errorf("fixedpoint query parameter expected")
			}
			key := p.scanner.ID()
			if err = p.advance(); err != nil {
				return false, err
			}
			if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
				return false, fmt.Errorf("invalid fixedpoint query parameter value")
			}
			value := p.scanner.ID() == "true"
			if err = p.advance(); err != nil {
				return false, err
			}
			switch key {
			case ":print-answer":
				printAnswer = value
			case ":print-statistics":
				printStatistics = value
			case ":print-certificate":
				printCertificate = value
			default:
				return false, fmt.Errorf("unsupported fixedpoint query parameter %s", key)
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if !p.ignoreChecks {
			started := time.Now()
			status, x := p.context.QueryFixedpoint(declaration)
			if x != nil {
				return false, x
			}
			text := "unknown"
			if status == lbool.True {
				text = "sat"
			} else if status == lbool.False {
				text = "unsat"
			}
			if _, err = fmt.Fprintln(p.out, text); err != nil {
				return false, err
			}
			if status == lbool.True && printAnswer {
				manager, y := p.context.Manager()
				if y != nil {
					return false, y
				}
				answer, y := p.context.FixedpointAnswer(declaration)
				if y != nil {
					return false, y
				}
				handle, y := manager.RefGet(answer)
				if y == nil {
					y = p.context.WriteExpression(p.out, handle)
				}
				if y == nil {
					_, y = fmt.Fprintln(p.out)
				}
				if y = errors.Join(y, manager.DelRef(&answer)); y != nil {
					return false, y
				}
			}
			if printCertificate {
				if _, x = fmt.Fprintln(p.out); x != nil {
					return false, x
				}
			}
			if printStatistics {
				stats, y := p.context.FixedpointStatistics()
				if y != nil {
					return false, y
				}
				stats.UpdateDouble("time", time.Since(started).Seconds())
				if y = stats.DisplaySMT2(p.out); y != nil {
					return false, y
				}
			}
		}
		success = false
	case "get-info":
		if p.token != KeywordToken {
			return false, fmt.Errorf("information keyword expected")
		}
		key := p.scanner.ID()
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		switch key {
		case ":all-statistics":
			err = p.context.WriteStatistics(p.out)
		case ":reason-unknown":
			err = p.context.WriteReasonUnknown(p.out)
		default:
			var value string
			var supported bool
			value, supported, err = p.context.GetInfo(key)
			if err == nil {
				if supported {
					_, err = fmt.Fprintln(p.out, value)
				} else {
					_, err = fmt.Fprintf(p.out, "; %s line: %d position: %d\nunsupported\n", key, commandLine, commandPosition)
				}
			}
		}
		if err != nil {
			return false, err
		}
		success = false
	case "get-option":
		if p.token != KeywordToken {
			return false, fmt.Errorf("option keyword expected")
		}
		keyword := p.scanner.ID()
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		value, supported, err := p.context.GetOption(keyword)
		if err != nil {
			return false, err
		}
		if supported {
			_, err = fmt.Fprintln(p.out, value)
		} else {
			_, err = fmt.Fprintf(p.out, "; %s line: %d position: %d\nunsupported\n", keyword, commandLine, commandPosition)
		}
		if err != nil {
			return false, err
		}
		success = false
	case "get-model":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteModel(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-assignment":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteAssignment(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-objectives":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteObjectives(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-value":
		values, text, err := p.valueExpressions()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteValuesWithText(p.out, values, text); err != nil {
			return false, err
		}
		success = false
	case "eval":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		options := cmd_context.EvaluationOptions{Timeout: ^uint32(0)}
		for p.token != RightParenToken {
			if p.token != KeywordToken {
				return false, fmt.Errorf("invalid keyword argument")
			}
			keyword := p.scanner.ID()
			if err = p.advance(); err != nil {
				return false, err
			}
			switch keyword {
			case ":completion", ":cache", ":array_equalities", ":array-equalities":
				if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
					return false, fmt.Errorf("invalid parameter value, true or false expected")
				}
				value := p.scanner.ID() == "true"
				switch keyword {
				case ":completion":
					options.Completion = &value
				case ":cache":
					options.Cache = &value
				default:
					options.ArrayEqualities = &value
				}
				if err = p.advance(); err != nil {
					return false, err
				}
			case ":max_memory", ":max-memory", ":max_steps", ":max-steps", ":timeout", ":model_index", ":model-index":
				value, e := p.unsigned()
				if e != nil {
					return false, e
				}
				switch keyword {
				case ":max_memory", ":max-memory":
					options.MaxMemory = &value
				case ":max_steps", ":max-steps":
					options.MaxSteps = &value
				case ":timeout":
					options.Timeout = value
				default:
					options.ModelIndex = value
				}
			default:
				return false, fmt.Errorf("invalid keyword argument")
			}
		}
		if err = p.context.WriteEvaluationWithOptions(p.out, expression, options); err != nil {
			return false, err
		}
		success = false
	case "get-consequences":
		assumptions, err := p.expressions()
		if err != nil {
			return false, err
		}
		variables, err := p.expressions()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteConsequences(p.out, assumptions, variables); err != nil {
			return false, err
		}
		success = false
	case "get-interpolant", "compute-interpolant":
		var targets []ast.ASTHandle
		for p.token != RightParenToken {
			target, _, e := p.expression()
			if e != nil {
				return false, e
			}
			targets = append(targets, target)
		}
		if len(targets) == 0 {
			return false, fmt.Errorf("not enough arguments")
		}
		tree := targets[0]
		for _, target := range targets[1:] {
			marked, e := p.pin(p.manager.MkInterp(tree))
			if e != nil {
				return false, e
			}
			tree, e = p.pin(p.manager.MkAnd(marked, target))
			if e != nil {
				return false, e
			}
		}
		if name == "get-interpolant" {
			err = p.context.WriteInterpolants(p.out, tree)
		} else {
			err = p.context.ComputeInterpolants(p.out, tree)
		}
		if err != nil {
			return false, err
		}
		success = false
	case "get-proof":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteProof(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-proof-graph":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteProofGraph(); err != nil {
			return false, err
		}
		success = false
	case "get-unsat-core":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteUnsatCore(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-unsat-assumptions":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteUnsatAssumptions(p.out); err != nil {
			return false, err
		}
		success = false
	case "labels":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteLabels(p.out); err != nil {
			return false, err
		}
		success = false
	case "get-assertions":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteAssertions(p.out); err != nil {
			return false, err
		}
		success = false
	case "echo":
		if p.token != StringToken {
			return false, fmt.Errorf("string expected")
		}
		text := p.scanner.String()
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if _, err = fmt.Fprintln(p.out, text); err != nil {
			return false, err
		}
		success = false
	case "display":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteExpression(p.out, expression); err != nil {
			return false, err
		}
		if _, err = fmt.Fprintln(p.out); err != nil {
			return false, err
		}
		success = false
	case "simplify":
		if err = p.simplifyCommand(); err != nil {
			return false, err
		}
		success = false
	case "reset", "reset-assertions":
		if p.token != RightParenToken {
			if name == "reset" {
				return false, fmt.Errorf("invalid reset command, ')' expected")
			}
			return false, fmt.Errorf("invalid command, too many arguments")
		}
		// Release parser-owned roots and its manager-bound shifter before
		// cmd_context::reset destroys the old AST manager.
		if err = p.clearCommand(); err != nil {
			return false, err
		}
		if name == "reset" || !p.context.GlobalDeclarations {
			if p.shifter != nil {
				if err = p.shifter.Close(); err != nil {
					return false, err
				}
				p.shifter = nil
			}
			p.manager = nil
		}
		if name == "reset" {
			err = p.context.Reset()
		} else {
			err = p.context.ResetAssertions()
		}
		if err != nil {
			return false, err
		}
		if p.context.PrintSuccess {
			if _, err = fmt.Fprintln(p.out, "success"); err != nil {
				return false, err
			}
		}
		return false, p.advance()
	case "exit":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if p.context.PrintSuccess {
			if _, err = fmt.Fprintln(p.out, "success"); err != nil {
				return false, err
			}
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported command %s", name)
	}
	if err = p.expect(RightParenToken); err != nil {
		return false, err
	}
	if success && p.context.PrintSuccess {
		_, err = fmt.Fprintln(p.out, "success")
	}
	return false, err
}

func (p *commandParser) skipSExpr() error {
	if p.token == EOFToken || p.token == RightParenToken {
		return fmt.Errorf("attribute value expected")
	}
	if p.token != LeftParenToken {
		return p.advance()
	}
	if err := p.advance(); err != nil {
		return err
	}
	for p.token != RightParenToken {
		if err := p.skipSExpr(); err != nil {
			return err
		}
	}
	return p.advance()
}

func (p *commandParser) expressions() ([]ast.ASTHandle, error) {
	if err := p.expect(LeftParenToken); err != nil {
		return nil, err
	}
	var values []ast.ASTHandle
	for p.token != RightParenToken {
		value, _, err := p.expression()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, p.advance()
}

func (p *commandParser) assertionExpression() (ast.ASTHandle, string, string, error) {
	if !p.context.InteractiveMode {
		expression, name, err := p.expression()
		return expression, name, "", err
	}
	compound := p.token == LeftParenToken
	p.scanner.StartCaching()
	defer func() {
		p.scanner.StopCaching()
		p.scanner.ResetCache()
	}()
	expression, name, err := p.expression()
	if err != nil {
		return ast.ASTHandle{}, "", "", err
	}
	text, err := p.scanner.CachedString(0, p.cacheEnd)
	if err != nil {
		return ast.ASTHandle{}, "", "", err
	}
	if compound {
		text = "(" + text
	}
	return expression, name, text, nil
}

// Source parse_get_value caches each original term before expanding
// definitions, lets, and coercions. Evaluate the AST, but echo that text.
func (p *commandParser) valueExpressions() ([]ast.ASTHandle, []string, error) {
	p.scanner.StartCaching()
	defer func() { p.scanner.StopCaching(); p.scanner.ResetCache() }()
	if err := p.expect(LeftParenToken); err != nil {
		return nil, nil, err
	}
	var values []ast.ASTHandle
	var text []string
	begin := 0
	for p.token != RightParenToken {
		value, _, err := p.expression()
		if err != nil {
			return nil, nil, err
		}
		ground, err := p.manager.IsGround(value)
		if err != nil {
			return nil, nil, err
		}
		quantifiers, err := p.manager.HasQuantifiers(value)
		if err != nil {
			return nil, nil, err
		}
		if !ground || quantifiers {
			return nil, nil, fmt.Errorf("invalid get-value term, term must be ground and must not contain quantifiers")
		}
		term, err := p.scanner.CachedString(begin, p.cacheEnd)
		if err != nil {
			return nil, nil, err
		}
		begin = p.cacheEnd
		values = append(values, value)
		text = append(text, term)
	}
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("invalid get-value command, empty list of terms")
	}
	return values, text, p.advance()
}

// parseIndexed consumes (_ name index...), used for sorts and applications.
func (p *commandParser) indexed() (string, []uint32, error) {
	name, err := p.symbol()
	if err != nil {
		return "", nil, err
	}
	var indices []uint32
	for p.token != RightParenToken {
		value, err := p.unsigned()
		if err != nil {
			return "", nil, err
		}
		indices = append(indices, value)
	}
	return name, indices, p.advance()
}

func (p *commandParser) sort() (ast.ASTHandle, error) {
	m, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token == SymbolToken {
		name, err := p.symbol()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if formal, found := p.sortParams[name]; found {
			return p.pin(formal, nil)
		}
		if arity, found := p.datatypeHeaders[name]; found {
			if arity != 0 {
				return ast.ASTHandle{}, fmt.Errorf("sort constructor expects %d parameters", arity)
			}
			return p.currentDatatypeSort(name, nil)
		}
		return p.pin(p.context.LookupSort(name))
	}
	if err := p.expect(LeftParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	name, err := p.symbol()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if name == "_" {
		name, indices, err := p.indexed()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !p.context.BuiltinSortAvailable(name) {
			return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
		}
		if name == "FloatingPoint" && len(indices) == 2 {
			return p.pin(m.FloatingPointSort(indices[0], indices[1]))
		}
		if name != "BitVec" || len(indices) != 1 {
			return ast.ASTHandle{}, fmt.Errorf("unsupported indexed sort %s", name)
		}
		return p.pin(m.BitVectorSort(indices[0]))
	}
	if name == "Array" {
		if !p.context.BuiltinSortAvailable(name) {
			return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
		}
		var sorts []ast.ASTHandle
		for p.token != RightParenToken {
			value, err := p.sort()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			sorts = append(sorts, value)
		}
		if err = p.expect(RightParenToken); err != nil {
			return ast.ASTHandle{}, err
		}
		if len(sorts) < 2 {
			return ast.ASTHandle{}, fmt.Errorf("array sort expects at least one index sort and a range sort")
		}
		return p.pin(m.MkArraySort(ast.SliceArguments(sorts[:len(sorts)-1]), sorts[len(sorts)-1]))
	}
	if name == "Seq" || name == "RegEx" {
		if !p.context.BuiltinSortAvailable(name) {
			return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
		}
		var sorts []ast.ASTHandle
		for p.token != RightParenToken {
			value, err := p.sort()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			sorts = append(sorts, value)
		}
		if len(sorts) != 1 {
			if name == "Seq" {
				return ast.ASTHandle{}, fmt.Errorf("Invalid sequence sort, expecting one parameter")
			}
			return ast.ASTHandle{}, fmt.Errorf("Invalid regex sort, expecting one parameter")
		}
		if err = p.expect(RightParenToken); err != nil {
			return ast.ASTHandle{}, err
		}
		family, err := m.SequenceFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		parameter, err := m.NewParameterAST(sorts[0])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		kind := int32(0)
		if name == "RegEx" {
			kind = 1
		}
		value, err := m.MkFamilySort(family, kind, []ast.ASTParameter{parameter})
		if err = errors.Join(err, m.DelParameter(&parameter)); err != nil {
			return ast.ASTHandle{}, err
		}
		return p.pin(value, nil)
	}
	var arguments []ast.ASTHandle
	for p.token != RightParenToken {
		value, e := p.sort()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments = append(arguments, value)
	}
	if err = p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	if arity, found := p.datatypeHeaders[name]; found {
		if uint32(len(arguments)) != arity {
			return ast.ASTHandle{}, fmt.Errorf("invalid number of parameters to sort constructor %s: got %d, want %d", name, len(arguments), arity)
		}
		return p.currentDatatypeSort(name, arguments)
	}
	return p.pin(p.context.InstantiateSort(name, arguments))
}

func (p *commandParser) indexedApplication(name string, indices []identifierIndex, args []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	parameters := make([]ast.ASTParameter, len(indices))
	defer func() {
		for i := range parameters {
			if parameters[i] != (ast.ASTParameter{}) {
				_ = p.manager.DelParameter(&parameters[i])
			}
		}
	}()
	for i, index := range indices {
		var err error
		if index.function.IsNull() {
			parameters[i], err = p.manager.NewParameterUint(index.number)
		} else {
			parameters[i], err = p.manager.NewParameterAST(index.function)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	ref, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	if err = p.context.MakeAppInto(name, args, parameters, rangeSort, ref); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.manager.RefGet(ref)
}

func parseBitVectorName(name string) (string, bool) {
	if !strings.HasPrefix(name, "bv") || len(name) == 2 {
		return "", false
	}
	for _, c := range name[2:] {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return name[2:], true
}

func (p *commandParser) parseBVNumeral(name string, indices []identifierIndex) (ast.ASTHandle, error) {
	text, ok := parseBitVectorName(name)
	if !ok || len(indices) != 1 || !indices[0].function.IsNull() {
		return ast.ASTHandle{}, fmt.Errorf("invalid bit-vector numeral")
	}
	// A decimal SMT-LIB bv numeral is integral and may exceed machine width.
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return ast.ASTHandle{}, fmt.Errorf("invalid bit-vector numeral")
	}
	return p.pin(p.manager.BitVectorNumeralRat(value, indices[0].number))
}
