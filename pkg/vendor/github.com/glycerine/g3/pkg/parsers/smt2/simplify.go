package smt2

// Source: src/cmd_context/simplify_cmd.cpp command-local parameter parsing and
// default simplified-term output.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *commandParser) simplifyCommand() (resultErr error) {
	expression, _, err := p.expression()
	if err != nil {
		return err
	}
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return err
	}
	settings, err := globals.Context.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, globals.Context.Del(&settings)) }()
	m, err := p.astManager()
	if err != nil {
		return err
	}
	descriptions, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, descriptions.Close()) }()
	if err = rewriter.TheoryRewriterParameterDescriptions(descriptions); err != nil {
		return err
	}
	printResult := true
	printProof := false
	printStatistics := false
	for p.token != RightParenToken && p.token != EOFToken {
		if p.token != KeywordToken {
			return fmt.Errorf("invalid simplify command, keyword expected")
		}
		key := strings.TrimPrefix(p.scanner.ID(), ":")
		key = uparams.NormParamName(&key)
		if err = p.advance(); err != nil {
			return err
		}
		if key == "timeout" {
			value, e := p.unsigned()
			if e != nil {
				return e
			}
			if e = globals.Context.SetUIntName(&settings, &key, value); e != nil {
				return e
			}
			continue
		}
		if key == "print" || key == "print_proofs" || key == "print_statistics" {
			if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
				return fmt.Errorf("invalid simplify parameter value, true or false expected")
			}
			if key == "print" {
				printResult = p.scanner.ID() == "true"
			} else if key == "print_proofs" {
				printProof = p.scanner.ID() == "true"
			} else {
				printStatistics = p.scanner.ID() == "true"
			}
			if err = p.advance(); err != nil {
				return err
			}
			continue
		}
		symbol, e := m.Symbols.Intern(key)
		if e != nil {
			return e
		}
		kind, e := descriptions.KindInModule(&symbol)
		if e != nil {
			return e
		}
		if kind == uparams.ParamInvalid {
			return fmt.Errorf("invalid simplify command, unknown parameter %s", key)
		}
		key, e = m.Symbols.String(symbol)
		if e != nil {
			return e
		}
		if err = p.tacticParameter(globals.Context, &settings, key, kind); err != nil {
			return err
		}
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	somName := "som"
	som, err := globals.Context.GetBoolName(&settings, &somName, false)
	if err != nil {
		return err
	}
	if som {
		flatName := "flat"
		if err = globals.Context.SetBoolName(&settings, &flatName, true); err != nil {
			return err
		}
	}
	result, proof, details, err := p.context.SimplifyCommandWithDetails(expression, &settings, printProof, printStatistics)
	if err != nil {
		return err
	}
	defer func() {
		if proof != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&proof))
		}
		resultErr = errors.Join(resultErr, m.DelRef(&result))
	}()
	if details.Failed {
		message := strings.ReplaceAll(details.Failure, "\"", "\"\"")
		if _, e := fmt.Fprintf(p.out, "(error \"simplifier failed: %s\")\n", message); e != nil {
			return e
		}
	}
	if printResult {
		simplified, e := m.RefGet(result)
		if e != nil {
			return e
		}
		if e = p.context.WriteExpression(p.out, simplified); e != nil {
			return e
		}
		if _, e = fmt.Fprintln(p.out); e != nil {
			return e
		}
	}
	if printProof && !details.Failed {
		proofExpression, e := m.RefGet(proof)
		if e != nil {
			return e
		}
		if e = p.context.WriteExpression(p.out, proofExpression); e != nil {
			return e
		}
		if _, e = fmt.Fprintln(p.out); e != nil {
			return e
		}
	}
	if printStatistics {
		megabyte := float64(1024 * 1024)
		_, err = fmt.Fprintf(p.out, "(:time %.2f :num-steps %d :memory %.2f :max-memory %.2f :cache-size: %d :num-nodes-before %d",
			details.Seconds, details.Steps, float64(details.MemoryBytes)/megabyte, float64(details.MaximumMemoryBytes)/megabyte,
			details.CacheSize, details.NodesBefore)
		if err == nil && !details.Failed {
			_, err = fmt.Fprintf(p.out, " :num-shared %d :num-nodes %d", details.SharedNodes, details.NodesAfter)
		}
		if err == nil {
			_, err = fmt.Fprintln(p.out, ")")
		}
		return err
	}
	return nil
}
