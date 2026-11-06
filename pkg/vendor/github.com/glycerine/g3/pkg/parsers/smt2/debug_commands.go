package smt2

// Source: src/cmd_context/extra_cmds/dbg_cmds.cpp, expression storage and size.

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *commandParser) debugCommand(name string) (resultErr error) {
	switch name {
	case "dbg-sexpr":
		text, err := p.debugSExpr()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out, text)
		return err
	case "display-dimacs":
		if err := p.requireEnd(); err != nil {
			return err
		}
		return p.context.DisplayDIMACS()
	case "dbg-translator":
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		return p.context.WriteDebugTranslation(p.out, expression)
	case "dbg-set-next-id":
		id, err := p.unsigned()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		return m.SetNextExprID(id)
	case "dbg-params":
		if err := p.requireEnd(); err != nil {
			return err
		}
		globals, err := p.context.GlobalParameters()
		if err != nil {
			return err
		}
		c := globals.Context
		first, err := c.NewValue()
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, c.Del(&first)) }()
		second, err := c.NewValue()
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, c.Del(&second)) }()
		key := "val"
		if err = c.SetUIntName(&first, &key, 100); err != nil {
			return err
		}
		if err = c.Copy(&second, &first); err != nil {
			return err
		}
		if err = c.SetUIntName(&second, &key, 200); err != nil {
			return err
		}
		firstValue, err := c.GetUIntName(&first, &key, 0)
		if err != nil {
			return err
		}
		secondValue, err := c.GetUIntName(&second, &key, 0)
		if err != nil {
			return err
		}
		if firstValue != 100 || secondValue != 200 {
			return fmt.Errorf("parameter copy-on-write check failed")
		}
		if err = c.Copy(&second, &first); err != nil {
			return err
		}
		secondValue, err = c.GetUIntName(&second, &key, 0)
		if err != nil {
			return err
		}
		if secondValue != 100 {
			return fmt.Errorf("parameter reassignment check failed")
		}
		_, err = fmt.Fprintln(p.out, "worked")
		return err
	case "dbg-lt":
		left, _, err := p.expression()
		if err != nil {
			return err
		}
		right, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		less, err := m.ASTLess(left, right)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out, less)
		return err
	case "dbg-some-value":
		sort, err := p.sort()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		value, err := m.GetSomeValue(sort)
		if err != nil {
			return err
		}
		if err = p.context.WriteExpression(p.out, value); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out)
		return err
	case "dbg-used-vars":
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		kind, err := m.GetKind(expression)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			expression, err = m.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
		}
		used, err := ast.NewASTUsedVars(m)
		if err != nil {
			return err
		}
		if err = used.Collect(expression); err != nil {
			return err
		}
		if _, err = fmt.Fprint(p.out, "(vars"); err != nil {
			return err
		}
		for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
			if _, err = fmt.Fprintf(p.out, "\n  (%-6d ", i); err != nil {
				return err
			}
			sort, err := used.Get(i)
			if err != nil {
				return err
			}
			if sort == (ast.ASTHandle{}) {
				_, err = fmt.Fprint(p.out, "<not-used>")
			} else {
				err = p.context.WriteSort(p.out, sort, 10)
			}
			if err != nil {
				return err
			}
			if _, err = fmt.Fprint(p.out, ")"); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(p.out, ")")
		return err
	case "dbg-bool-rewriter", "dbg-bool-flat-rewriter", "dbg-elim-and":
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		globals, err := p.context.GlobalParameters()
		if err != nil {
			return err
		}
		params, err := globals.Context.NewValue()
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, globals.Context.Del(&params)) }()
		flat := "flat"
		if err = globals.Context.SetBoolName(&params, &flat, name != "dbg-bool-rewriter"); err != nil {
			return err
		}
		if name == "dbg-elim-and" {
			key := "elim_and"
			if err = globals.Context.SetBoolName(&params, &key, true); err != nil {
				return err
			}
		}
		rw, err := rewriter.NewBoolRewriterStar(m, globals.Context, globals, &params)
		if rw != nil {
			defer func() { resultErr = errors.Join(resultErr, rw.Close()) }()
		}
		if err != nil {
			return err
		}
		result, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.DelRef(&result)) }()
		if err = rw.Rewrite(expression, result); err != nil {
			return err
		}
		value, err := m.RefGet(result)
		if err != nil {
			return err
		}
		if err = p.context.WriteExpression(p.out, value); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out)
		return err
	case "dbg-elim-unused-vars":
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		kind, err := m.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			return p.context.WriteExpression(p.out, expression)
		}
		globals, err := p.context.GlobalParameters()
		if err != nil {
			return err
		}
		params, err := globals.SourceGetRef()
		if err != nil {
			return err
		}
		result, err := rewriter.ElimUnusedVarsWithParams(m, expression, globals.Context, params)
		if result != (ast.ASTRef{}) {
			defer func() { resultErr = errors.Join(resultErr, m.DelRef(&result)) }()
		}
		if err != nil {
			return err
		}
		value, err := m.RefGet(result)
		if err != nil {
			return err
		}
		if err = p.context.WriteExpression(p.out, value); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out)
		return err
	case "dbg-instantiate", "dbg-instantiate-nested":
		quantifier, _, err := p.expression()
		if err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		kind, err := m.GetKind(quantifier)
		if err != nil || kind != ast.ASTQuantifier {
			return fmt.Errorf("invalid command, quantifier expected")
		}
		if name == "dbg-instantiate-nested" {
			quantifier, err = m.GetQuantifierBody(quantifier)
			if err != nil {
				return err
			}
			kind, err = m.GetKind(quantifier)
			if err != nil || kind != ast.ASTQuantifier {
				return fmt.Errorf("invalid command, nested quantifier expected")
			}
		}
		if err = p.expect(LeftParenToken); err != nil {
			return err
		}
		var args []ast.ASTHandle
		for p.token != RightParenToken {
			value, _, err := p.expression()
			if err != nil {
				return err
			}
			args = append(args, value)
		}
		if err = p.advance(); err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		num, err := m.GetNumDeclarations(quantifier)
		if err != nil {
			return err
		}
		if uint64(len(args)) != uint64(num) {
			return fmt.Errorf("invalid command, mismatch between the number of quantified variables and the number of arguments")
		}
		for i := range args {
			actual, err := m.GetSort(args[i])
			if err != nil {
				return err
			}
			expected, err := m.GetDeclarationSort(quantifier, uint32(i))
			if err != nil {
				return err
			}
			if actual != expected {
				return fmt.Errorf("invalid command, sort mismatch at position %d", i)
			}
		}
		result, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.DelRef(&result)) }()
		if err = rewriter.InstantiateQuantifier(m, quantifier, args, result); err != nil {
			return err
		}
		value, err := m.RefGet(result)
		if err != nil {
			return err
		}
		if err = p.context.WriteExpression(p.out, value); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out)
		return err
	case "dbg-subst":
		source, err := p.symbol()
		if err != nil {
			return err
		}
		term, err := p.context.DebugExpression(source)
		if err != nil {
			return err
		}
		if err = p.expect(LeftParenToken); err != nil {
			return err
		}
		var args []ast.ASTHandle
		for p.token != RightParenToken {
			key, err := p.symbol()
			if err != nil {
				return err
			}
			value, err := p.context.DebugExpression(key)
			if err != nil {
				return err
			}
			args = append(args, value)
		}
		if err = p.advance(); err != nil {
			return err
		}
		target, err := p.symbol()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		subst, err := rewriter.NewVarSubstitution(m, false)
		if subst != nil {
			defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
		}
		if err != nil {
			return err
		}
		result, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.DelRef(&result)) }()
		if err = subst.Substitute(term, args, result); err != nil {
			return err
		}
		value, err := m.RefGet(result)
		if err != nil {
			return err
		}
		return p.context.StoreDebugExpression(target, value)
	case "dbg-set", "dbg-get-qbody":
		key, err := p.symbol()
		if err != nil {
			return err
		}
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		if name == "dbg-get-qbody" {
			m, err := p.astManager()
			if err != nil {
				return err
			}
			expression, err = m.GetQuantifierBody(expression)
			if err != nil {
				return fmt.Errorf("invalid command, term must be a quantifier")
			}
		}
		return p.context.StoreDebugExpression(key, expression)
	case "dbg-pp-var":
		key, err := p.symbol()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		expression, err := p.context.DebugExpression(key)
		if err != nil {
			return err
		}
		if err = p.context.WriteExpression(p.out, expression); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out)
		return err
	case "dbg-shift-vars":
		key, err := p.symbol()
		if err != nil {
			return err
		}
		shift, err := p.unsigned()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		expression, err := p.context.DebugExpression(key)
		if err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		shifter, err := rewriter.NewVarShifter(m)
		if shifter != nil {
			defer func() { resultErr = errors.Join(resultErr, shifter.Close()) }()
		}
		if err != nil {
			return err
		}
		result, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.DelRef(&result)) }()
		if err = shifter.ShiftBy(expression, shift, result); err != nil {
			return err
		}
		expression, err = m.RefGet(result)
		if err != nil {
			return err
		}
		return p.context.StoreDebugExpression(key, expression)
	case "dbg-size", "dbg-num-shared", "dbg-pp-shared":
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		var count uint32
		if name == "dbg-size" {
			count, err = m.GetNumExprs(expression)
		} else {
			shared, e := ast.NewASTSharedOccurrences(m)
			if e != nil {
				return e
			}
			defer func() { resultErr = errors.Join(resultErr, shared.Close()) }()
			if err = shared.Collect(expression); err == nil {
				count = shared.NumShared()
			}
			if err != nil {
				return err
			}
			if name == "dbg-pp-shared" {
				if _, err = fmt.Fprint(p.out, "(shared"); err != nil {
					return err
				}
				iterator, err := shared.BeginShared()
				if err != nil {
					return err
				}
				for {
					done, err := iterator.Done()
					if err != nil {
						return err
					}
					if done {
						break
					}
					value, err := iterator.Value()
					if err != nil {
						return err
					}
					if _, err = fmt.Fprint(p.out, "\n  "); err != nil {
						return err
					}
					if err = p.context.WriteExpressionIndented(p.out, value, 2); err != nil {
						return err
					}
					if err = iterator.Next(); err != nil {
						return err
					}
				}
				_, err = fmt.Fprintln(p.out, ")")
				return err
			}
		}
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out, count)
		return err
	}
	return fmt.Errorf("unknown debug command %s", name)
}

func (p *commandParser) debugSExpr() (string, error) {
	if p.token == EOFToken || p.token == RightParenToken {
		return "", fmt.Errorf("s-expression expected")
	}
	if p.token == LeftParenToken {
		if err := p.advance(); err != nil {
			return "", err
		}
		var elements []string
		for p.token != RightParenToken {
			if p.token == EOFToken {
				return "", fmt.Errorf("unexpected end of s-expression")
			}
			element, err := p.debugSExpr()
			if err != nil {
				return "", err
			}
			elements = append(elements, element)
		}
		if err := p.advance(); err != nil {
			return "", err
		}
		return "(" + strings.Join(elements, " ") + ")", nil
	}
	var text string
	switch p.token {
	case StringToken:
		text = `"` + strings.ReplaceAll(p.scanner.String(), `"`, `""`) + `"`
	case KeywordToken, SymbolToken:
		text = p.scanner.ID()
	case IntegerToken:
		text = p.scanner.Number().Num().String()
	case DecimalToken:
		value := p.scanner.Number()
		scale := decimalScale(value.Denom())
		text = value.FloatString(scale)
	case BitVectorToken:
		text = fmt.Sprintf("(_ bv%s %d)", p.scanner.Number().Num().String(), p.scanner.BitVectorSize())
	default:
		return "", fmt.Errorf("s-expression atom expected")
	}
	if err := p.advance(); err != nil {
		return "", err
	}
	return text, nil
}

func decimalScale(denominator *big.Int) int {
	remaining := new(big.Int).Set(denominator)
	two, five := big.NewInt(2), big.NewInt(5)
	var twos, fives int
	for new(big.Int).Mod(remaining, two).Sign() == 0 {
		remaining.Div(remaining, two)
		twos++
	}
	for new(big.Int).Mod(remaining, five).Sign() == 0 {
		remaining.Div(remaining, five)
		fives++
	}
	if remaining.Cmp(big.NewInt(1)) != 0 {
		return 0
	}
	if twos > fives {
		return twos
	}
	return fives
}
