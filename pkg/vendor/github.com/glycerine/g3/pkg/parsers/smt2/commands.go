package smt2

// Source: src/parsers/smt2/smt2parser.cpp command dispatch and retained
// declaration/assertion/check commands; src/cmd_context/basic_cmds.cpp.

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/escaped"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type commandParser struct {
	context            *cmd_context.Context
	scanner            *Scanner
	filename           string
	freshResetSortGap  bool
	token              Token
	parenDepth         int
	cacheEnd           int
	out                io.Writer
	manager            *ast.ASTManager
	refs               []ast.ASTRef
	env                smt2LocalEnv
	exprs              localExprStack
	locals             localExprStack
	sorts              localSortStack
	bindings           uint32
	shifter            *rewriter.VarShifter
	sortParams         map[string]ast.ASTHandle
	datatypeHeaders    map[string]uint32
	ignoreUserPatterns bool
	ignoreBadPatterns  bool
	ignoreChecks       bool
	queryCollector     *fixedpointQueryCollector
	tacticExpansion    []string
	softWeight         big.Rat
	softID             string
}

// ReportedParseError indicates that ParseShellCommands already emitted the
// source parser's error response and recovered through the rest of the input.
type ReportedParseError struct{ cause error }

func (e *ReportedParseError) Error() string { return e.cause.Error() }
func (e *ReportedParseError) Unwrap() error { return e.cause }

// ParseCommands executes the retained SMT-LIB command grammar against Context.
// Unsupported commands fail explicitly. Command-local AST references are
// released after execution; the context owns declarations and assertions.
func ParseCommands(context *cmd_context.Context, input io.Reader, out io.Writer) (result error) {
	return parseCommands(context, input, out, false, nil)
}

// ParseShellCommands uses the shell's default stdout identity for its output.
func ParseShellCommands(context *cmd_context.Context, input io.Reader, out io.Writer) error {
	return parseCommandsWithOutput(context, input, out, false, nil, true)
}

// ParseAssertions parses a solver-loading SMT-LIB stream. Check commands are
// syntax-checked but not executed or printed; callers consume the context's
// final assertion set only, matching api_solver.cpp solver_from_stream.
func ParseAssertions(context *cmd_context.Context, input io.Reader) error {
	return parseCommands(context, input, io.Discard, true, nil)
}

func parseCommands(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector) (result error) {
	return parseCommandsWithOutput(context, input, out, ignoreChecks, collector, false)
}

func parseCommandsWithOutput(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector, shell bool) (result error) {
	return parseCommandsWithOptions(context, input, out, ignoreChecks, collector, shell, "", shell)
}

func parseCommandsWithOptions(context *cmd_context.Context, input io.Reader, out io.Writer, ignoreChecks bool, collector *fixedpointQueryCollector, shell bool, filename string, reportErrors bool) (result error) {
	var err error
	if shell {
		err = context.SetShellRegularOutput(out)
	} else {
		err = context.SetRegularOutput(out)
	}
	if err != nil {
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
	p := &commandParser{context: context, scanner: scanner, filename: filename, out: context.RegularOutput(), ignoreChecks: ignoreChecks, queryCollector: collector}
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
	var firstParseError error
	if err = p.advance(); err != nil {
		firstParseError = parserScannerError(err, scanner)
		report := func(cause error) error { return nil }
		if reportErrors {
			report = func(cause error) error { return reportParserError(p, cause) }
		}
		if err = report(firstParseError); err != nil {
			return errors.Join(firstParseError, err)
		}
		if err = p.syncAfterError(report); err != nil {
			return errors.Join(firstParseError, err)
		}
	}
	for p.token != EOFToken {
		exit, err := p.command()
		if err != nil {
			parseErr := parserScannerError(err, scanner)
			if firstParseError == nil {
				firstParseError = parseErr
			}
			report := func(cause error) error { return nil }
			if reportErrors {
				report = func(cause error) error { return reportParserError(p, cause) }
			}
			if err = report(parseErr); err != nil {
				return errors.Join(firstParseError, err)
			}
			if err = p.clearCommand(); err != nil {
				return errors.Join(firstParseError, err)
			}
			if p.shifter != nil {
				if err = p.shifter.Close(); err != nil {
					return errors.Join(firstParseError, err)
				}
				p.shifter = nil
			}
			if err = p.syncAfterError(func(cause error) error {
				if firstParseError == nil {
					firstParseError = cause
				}
				return report(cause)
			}); err != nil {
				return errors.Join(firstParseError, err)
			}
			continue
		}
		if err = p.clearCommand(); err != nil {
			return err
		}
		if exit {
			if firstParseError != nil && reportErrors {
				return &ReportedParseError{cause: firstParseError}
			}
			return firstParseError
		}
	}
	if firstParseError != nil {
		if reportErrors {
			return &ReportedParseError{cause: firstParseError}
		}
		return firstParseError
	}
	return nil
}

func (p *commandParser) advance() error {
	switch p.token {
	case LeftParenToken:
		p.parenDepth++
	case RightParenToken:
		p.parenDepth--
	}
	p.cacheEnd = p.scanner.CacheSize()
	var err error
	p.token, err = p.scanner.Scan()
	return err
}

func parserScannerError(err error, scanner *Scanner) *ScannerError {
	var parseError *ScannerError
	if errors.As(err, &parseError) {
		return parseError
	}
	return &ScannerError{Message: err.Error(), Line: scanner.Line(), Column: scanner.Position()}
}

func reportParserError(p *commandParser, err error) error {
	var parseError *ScannerError
	if !errors.As(err, &parseError) {
		parseError = &ScannerError{Message: err.Error(), Line: p.scanner.Line(), Column: p.scanner.Position()}
	}
	message, cause := escaped.String(&parseError.Message, true, 0)
	if cause != nil {
		return cause
	}
	if p.filename != "" {
		_, cause = fmt.Fprintf(p.out, "(error \"%s: line %d column %d: %s\")\n", p.filename, parseError.Line, parseError.Column, message)
	} else {
		_, cause = fmt.Fprintf(p.out, "(error \"line %d column %d: %s\")\n", parseError.Line, parseError.Column, message)
	}
	return cause
}

func (p *commandParser) syncAfterError(report func(error) error) error {
	if p.parenDepth < 0 {
		p.parenDepth = 0
	}
	for p.token != EOFToken {
		if p.parenDepth == 0 && p.token == LeftParenToken {
			return nil
		}
		if err := p.advance(); err != nil {
			if cause := report(err); cause != nil {
				return cause
			}
		}
		if p.parenDepth < 0 {
			p.parenDepth = 0
		}
	}
	return nil
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
	p.freshResetSortGap = false
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
		result = errors.Join(result, p.sorts.reset(p.manager))
		result = errors.Join(result, p.exprs.reset(p.manager))
		result = errors.Join(result, p.locals.reset(p.manager))
		for i := len(p.refs) - 1; i >= 0; i-- {
			result = errors.Join(result, p.manager.DelRef(&p.refs[i]))
		}
	}
	p.refs = nil
	p.env.reset()
	p.bindings = 0
	p.tacticExpansion = nil
	return result
}

func (p *commandParser) fixedpointQuery(declaration ast.ASTHandle) (status lbool.LBool, printAnswer, printStatistics, printCertificate bool, resultErr error) {
	if err := p.validateFixedpointQueryDeclaration(declaration); err != nil {
		return lbool.Undef, false, false, false, err
	}
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	context := globals.Context
	options, err := context.NewValue()
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&options)) }()
	descriptions, err := globals.SourceModuleDescriptorsName("fixedpoint")
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	for p.token != RightParenToken {
		if p.token != KeywordToken {
			return lbool.Undef, false, false, false, fmt.Errorf("fixedpoint query parameter expected")
		}
		rawKey := p.scanner.ID()
		key := uparams.NormParamName(&rawKey)
		if err = p.advance(); err != nil {
			return lbool.Undef, false, false, false, err
		}
		kind, x := descriptions.KindName(key)
		if x != nil {
			return lbool.Undef, false, false, false, x
		}
		if kind == uparams.ParamInvalid {
			return lbool.Undef, false, false, false, fmt.Errorf("unsupported fixedpoint query parameter :%s", key)
		}
		if err = p.tacticParameter(context, &options, key, kind); err != nil {
			return lbool.Undef, false, false, false, err
		}
	}
	if err = p.requireEnd(); err != nil {
		return lbool.Undef, false, false, false, err
	}
	module, err := globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return lbool.Undef, false, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&module)) }()
	answerName, statisticsName, certificateName := "print_answer", "print_statistics", "print_certificate"
	printAnswer, err = context.GetBoolFallbackName(&options, &answerName, &module, false)
	if err == nil {
		printStatistics, err = context.GetBoolFallbackName(&options, &statisticsName, &module, false)
	}
	if err == nil {
		printCertificate, err = context.GetBoolFallbackName(&options, &certificateName, &module, false)
	}
	if err != nil || p.ignoreChecks {
		return lbool.Undef, printAnswer, printStatistics, printCertificate, err
	}
	status, err = p.context.QueryFixedpointCommandWithParams(declaration, &options)
	return status, printAnswer, printStatistics, printCertificate, err
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
	case "dbg-set", "dbg-get-qbody", "dbg-pp-var", "dbg-size", "dbg-shift-vars", "dbg-num-shared", "dbg-pp-shared", "dbg-subst", "dbg-instantiate", "dbg-instantiate-nested", "dbg-elim-unused-vars", "dbg-bool-rewriter", "dbg-bool-flat-rewriter", "dbg-elim-and", "dbg-used-vars", "dbg-lt", "dbg-some-value", "dbg-params", "dbg-set-next-id", "dbg-translator", "dbg-sexpr", "display-dimacs":
		if err = p.debugCommand(name); err != nil {
			return false, err
		}
		success = false
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
		if p.token == DecimalToken || p.token == BitVectorToken {
			value := p.scanner.Number()
			if err = p.advance(); err != nil {
				return false, err
			}
			if err = p.requireEnd(); err != nil {
				return false, err
			}
			if err = p.context.SetNumeralOption(key, value); err != nil {
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
		var representations []string
		for p.token != RightParenToken {
			representation, x := p.symbol()
			if x != nil {
				return false, x
			}
			representations = append(representations, representation)
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		declaration, x := p.context.DeclareRelation(relation, domain)
		if x != nil {
			return false, x
		}
		if err = p.context.SetFixedpointPredicateRepresentation(declaration, representations); err != nil {
			return false, err
		}
		if p.queryCollector != nil {
			if err = p.queryCollector.addRelation(p, declaration); err != nil {
				return false, err
			}
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
		name := symbol.Symbol{}
		bound := ^uint32(0)
		if p.token != RightParenToken {
			text, err := p.symbol()
			if err != nil {
				return false, err
			}
			manager, err := p.astManager()
			if err != nil {
				return false, err
			}
			// A supplied quoted empty symbol is distinct from an absent name.
			// SMT-LIB symbol tokens are strings, including the spelling k!7.
			name, err = manager.Symbols.Intern(text)
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
		// dl_rule_cmd expects symbols for every argument after the bound;
		// each one replaces the previous name.
		for p.token != RightParenToken {
			text, err := p.symbol()
			if err != nil {
				return false, err
			}
			manager, err := p.astManager()
			if err != nil {
				return false, err
			}
			name, err = manager.Symbols.Intern(text)
			if err != nil {
				return false, err
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if p.queryCollector != nil {
			if err = p.queryCollector.addRule(p, expression, name); err != nil {
				return false, err
			}
		} else if err = p.context.AddFixedpointRuleWithSymbolAndBound(expression, name, bound); err != nil {
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
		if p.queryCollector != nil {
			if name == "fixedpoint-push" {
				p.queryCollector.push(1)
			} else if err = p.queryCollector.pop(1); err != nil {
				return false, err
			}
		}
	case "assert-soft":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		m, err := p.astManager()
		if err != nil {
			return false, err
		}
		sort, err := m.GetSort(expression)
		if err != nil {
			return false, err
		}
		boolean, err := m.MkBoolSort()
		if err != nil {
			return false, err
		}
		if sort != boolean {
			return false, fmt.Errorf("invalid type for expression: expected Boolean type")
		}
		weight := new(big.Rat).Set(&p.softWeight)
		id := p.softID
		for p.token != RightParenToken {
			if p.token != KeywordToken {
				return false, fmt.Errorf("assert-soft option keyword expected")
			}
			rawKey := p.scanner.ID()
			key := uparams.NormParamName(&rawKey)
			if err = p.advance(); err != nil {
				return false, err
			}
			switch key {
			case "weight":
				weight, err = p.rational()
				if err == nil {
					p.softWeight.Set(weight)
				}
			case "id":
				id, err = p.symbol()
				if err == nil {
					p.softID = id
				}
			default:
				return false, fmt.Errorf("unsupported assert-soft option %s", rawKey)
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
		// The source registers the objective in set_next_arg, before the
		// parser diagnoses an extra command argument.
		if err = p.context.AddObjective(expression, name == "maximize"); err != nil {
			return false, err
		}
		if p.context.PrintSuccess {
			if _, err = fmt.Fprintln(p.out, "success"); err != nil {
				return false, err
			}
		}
		success = false
		if err = p.requireEnd(); err != nil {
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
		if !p.ignoreChecks {
			p.freshResetSortGap = false
		}
		success = false
	case "declare-tactic":
		name, err := p.symbol()
		if err != nil {
			return false, err
		}
		selected, body, err := p.parseTacticDeclaration()
		if err != nil {
			return false, err
		}
		if err = selected.DecRef(); err != nil {
			return false, err
		}
		if err = p.context.DeclareUserTactic(name, body); err != nil {
			return false, err
		}
	case "apply":
		if err := p.applyTactic(!p.ignoreChecks); err != nil {
			return false, err
		}
		if !p.ignoreChecks {
			p.freshResetSortGap = false
		}
		success = false
	case "check-sat", "check-sat-assuming":
		if err := p.checkSatCommand(name == "check-sat-assuming", !p.ignoreChecks); err != nil {
			return false, err
		}
		if !p.ignoreChecks {
			p.freshResetSortGap = false
		}
		success = false
	case "query":
		declaration, err := p.functionDeclarationReference()
		if err != nil {
			return false, err
		}
		started := time.Now()
		status, printAnswer, printStatistics, printCertificate, err := p.fixedpointQuery(declaration)
		queryException := cmd_context.FixedpointCaughtQueryException(err)
		if err != nil {
			if queryException == nil {
				return false, err
			}
			if _, err = fmt.Fprintf(p.out, "(error \"query failed: %s\")\n", queryException.Message); err != nil {
				return false, err
			}
		}
		if p.queryCollector != nil {
			if err = p.queryCollector.add(p, declaration); err != nil {
				return false, err
			}
		}
		if !p.ignoreChecks {
			text := "unknown"
			reason := ""
			bounded := false
			if status == lbool.True {
				text = "sat"
			} else if status == lbool.False {
				text = "unsat"
			} else if queryException == nil {
				var reasonErr error
				reason, reasonErr = p.context.FixedpointReasonUnknown()
				if reasonErr != nil {
					return false, reasonErr
				}
				if reason == "bounded" {
					bounded = true
					text = "bounded"
				}
			}
			if _, err = fmt.Fprintln(p.out, text); err != nil {
				return false, err
			}
			if status == lbool.Undef && !bounded {
				detail := ""
				switch reason {
				case "input error", "memory bounds exceeded", "timeout", "canceled":
					detail = reason
				case "approximated":
					detail = "approximated relations"
				}
				if detail != "" {
					if _, err = fmt.Fprintln(p.out, detail); err != nil {
						return false, err
					}
				}
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
				cleanupErr := manager.DelRef(&answer)
				if answer != (ast.ASTRef{}) {
					return false, &fixedpointAnswerCleanupFailure{manager: manager, reference: answer, cause: y, cleanupErr: cleanupErr}
				}
				if y = errors.Join(y, cleanupErr); y != nil {
					return false, y
				}
			}
			if printCertificate && (status != lbool.Undef || bounded) {
				_, y := p.context.WriteFixedpointCertificate(p.out, declaration)
				if y != nil {
					return false, y
				}
				if _, y = fmt.Fprintln(p.out); y != nil {
					return false, y
				}
			}
			if err = p.context.ResetFixedpointCommandStatus(); err != nil {
				return false, err
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
	case "get-user-tactics":
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		var declarations strings.Builder
		for _, declaration := range p.context.UserTacticDeclarations() {
			if declarations.Len() != 0 {
				declarations.WriteByte('\n')
			}
			fmt.Fprintf(&declarations, "(declare-tactic %s %s)", declaration.Name, declaration.Body)
		}
		text := strings.ReplaceAll(declarations.String(), "\"", "\\\"")
		if _, err = fmt.Fprintf(p.out, "(%s)\n", text); err != nil {
			return false, err
		}
		success = false
	case "help-tactic":
		if err = p.helpTactic(); err != nil {
			return false, err
		}
		success = false
	case "help":
		if err = p.helpCommands(); err != nil {
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
		modelIndex := uint32(0)
		for p.token != RightParenToken {
			if p.token != IntegerToken {
				return false, fmt.Errorf("invalid command argument, unsigned integer expected")
			}
			modelIndex, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if err = p.context.WriteModelAtModelIndex(p.out, modelIndex); err != nil {
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
		modelIndex := uint32(0)
		if p.token == KeywordToken && (p.scanner.ID() == ":model-index" || p.scanner.ID() == ":model_index") {
			if err = p.advance(); err != nil {
				return false, err
			}
			if p.token != IntegerToken {
				return false, fmt.Errorf("integer index expected to indexed model evaluation")
			}
			modelIndex, err = p.unsigned()
			if err != nil {
				return false, err
			}
		}
		if p.token != RightParenToken {
			return false, fmt.Errorf("invalid get-value command, ')' expected")
		}
		if err = p.context.WriteValuesWithTextAtModelIndex(p.out, values, text, modelIndex); err != nil {
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
			keyword, e := p.parametricKeyword()
			if e != nil {
				return false, e
			}
			if err = p.advance(); err != nil {
				return false, err
			}
			switch keyword {
			case "completion", "cache", "array_equalities":
				if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
					return false, fmt.Errorf("invalid parameter value, true or false expected")
				}
				value := p.scanner.ID() == "true"
				switch keyword {
				case "completion":
					options.Completion = &value
				case "cache":
					options.Cache = &value
				default:
					options.ArrayEqualities = &value
				}
				if err = p.advance(); err != nil {
					return false, err
				}
			case "max_memory", "max_steps", "timeout", "rlimit", "model_index":
				value, e := p.unsigned()
				if e != nil {
					return false, e
				}
				switch keyword {
				case "max_memory":
					options.MaxMemory = &value
				case "max_steps":
					options.MaxSteps = &value
				case "timeout":
					options.Timeout = value
				case "rlimit":
					options.RLimit = value
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
		if err = p.getConsequencesCommand(!p.ignoreChecks); err != nil {
			return false, err
		}
		success = false
	case "include":
		if p.token != StringToken {
			return false, fmt.Errorf("invalid command argument, string expected")
		}
		filename := p.scanner.String()
		if err = p.advance(); err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		file, openErr := os.Open(filename)
		if openErr != nil {
			return false, fmt.Errorf("failed to open file '%s'", filename)
		}
		parseErr := parseCommandsWithOptions(p.context, file, p.out, false, nil, false, filename, true)
		closeErr := file.Close()
		var reported *ReportedParseError
		if errors.As(parseErr, &reported) {
			parseErr = nil
		}
		if parseErr != nil || closeErr != nil {
			return false, errors.Join(parseErr, closeErr)
		}
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
		if err = p.echoCommand(); err != nil {
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
	case "to-subpaving":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WriteSubpaving(p.out, expression); err != nil {
			return false, err
		}
		success = false
	case "to-poly":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WritePolynomial(p.out, expression); err != nil {
			return false, err
		}
		success = false
	case "poly/isolate-roots":
		if p.token == RightParenToken {
			return false, fmt.Errorf("polynomial expected")
		}
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		var assignments []ast.ASTHandle
		for p.token != RightParenToken {
			argument, _, err := p.expression()
			if err != nil {
				return false, err
			}
			assignments = append(assignments, argument)
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WritePolynomialRoots(p.out, expression, assignments); err != nil {
			return false, err
		}
		success = false
	case "poly/factor":
		expression, _, err := p.expression()
		if err != nil {
			return false, err
		}
		params := polynomial.DefaultUPolynomialFactorParams()
		for p.token == KeywordToken {
			key, e := p.parametricKeyword()
			if e != nil {
				return false, e
			}
			if key != "max_prime" && key != "num_primes" && key != "max_search_size" {
				return false, fmt.Errorf("invalid keyword argument")
			}
			if err = p.advance(); err != nil {
				return false, err
			}
			if p.token != IntegerToken && p.token != DecimalToken {
				return false, fmt.Errorf("invalid parameter value, unsigned integer expected")
			}
			number := p.scanner.Number()
			if !number.IsInt() || number.Sign() < 0 || !number.Num().IsUint64() || number.Num().Uint64() > uint64(^uint32(0)) {
				return false, fmt.Errorf("invalid parameter value, unsigned integer expected")
			}
			value := uint32(number.Num().Uint64())
			switch key {
			case "max_prime":
				params.MaxPrime = value
			case "num_primes":
				params.PrimeTrials = value
			case "max_search_size":
				params.MaxSearchSize = value
			}
			if err = p.advance(); err != nil {
				return false, err
			}
		}
		if err = p.requireEnd(); err != nil {
			return false, err
		}
		if err = p.context.WritePolynomialFactors(p.out, expression, params); err != nil {
			return false, err
		}
		success = false
	case "simplify", "dbg-th-rewriter":
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
		if p.queryCollector != nil && !p.context.BorrowsManager() && (name == "reset" || !p.context.GlobalDeclarations) {
			return false, fmt.Errorf("reset while collecting fixedpoint queries is not supported")
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
			p.freshResetSortGap = false
		} else {
			err = p.context.ResetAssertions()
			p.freshResetSortGap = !p.context.GlobalDeclarations
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
		// Source parse_unknown_cmd consumes the command's arguments, reports
		// the end location on the diagnostic stream, and emits "unsupported"
		// on the regular stream while continuing with the next command.
		for p.token != RightParenToken {
			if err = p.skipSExpr(); err != nil {
				return false, err
			}
		}
		if _, err = fmt.Fprintf(p.context.DiagnosticOutput(), "; %s line: %d position: %d\n", name, p.scanner.Line(), p.scanner.Position()); err != nil {
			return false, err
		}
		if _, err = fmt.Fprintln(p.out, "unsupported"); err != nil {
			return false, err
		}
		return false, p.advance()
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

// qualifiedIdentifier parses the identifier portion accepted by source
// parse_indexed_identifier: either a symbol or an indexed `(_ name ...)`.
func (p *commandParser) qualifiedIdentifier() (string, []identifierIndex, error) {
	if p.token == SymbolToken {
		name, err := p.symbol()
		return name, nil, err
	}
	if err := p.expect(LeftParenToken); err != nil {
		return "", nil, err
	}
	marker, err := p.symbol()
	if err != nil {
		return "", nil, err
	}
	if marker != "_" {
		return "", nil, fmt.Errorf("invalid indexed identifier, '_' expected")
	}
	return p.indexedExpression()
}

func (p *commandParser) sort() (ast.ASTHandle, error) {
	// cmd_context::reset_assertions falls through to reset(false) when global
	// declarations are disabled. The native extension parser then performs
	// sort-name lookup before lazy manager initialization, so its next Int sort
	// lookup fails until another AST operation initializes the manager.
	if p.freshResetSortGap && p.token == SymbolToken && p.scanner.ID() == "Int" {
		return ast.ASTHandle{}, fmt.Errorf("unknown sort 'Int'")
	}
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
	last := len(p.refs) - 1
	p.refs[last] = ast.ASTRef{}
	p.refs = p.refs[:last]
	p.exprs.pushOwned(ref)
	return p.manager.RefGet(ref)
}

func parseBitVectorName(name string) (string, bool) {
	base, digits := 10, ""
	switch {
	case strings.HasPrefix(name, "bvbin"):
		base, digits = 2, name[len("bvbin"):]
	case strings.HasPrefix(name, "bvhex"):
		base, digits = 16, name[len("bvhex"):]
	case strings.HasPrefix(name, "bv"):
		digits = name[len("bv"):]
	default:
		return "", false
	}
	if digits == "" {
		return "", false
	}
	for _, digit := range digits {
		valid := '0' <= digit && digit <= '9'
		if base == 2 {
			valid = digit == '0' || digit == '1'
		} else if base == 16 {
			valid = ('0' <= digit && digit <= '9') || ('a' <= digit && digit <= 'f') || ('A' <= digit && digit <= 'F')
		}
		if !valid {
			return "", false
		}
	}
	value, ok := new(big.Int).SetString(digits, base)
	if !ok || value.Sign() < 0 {
		return "", false
	}
	return value.String(), true
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
	return p.pinExpression(p.manager.BitVectorNumeralRat(value, indices[0].number))
}
