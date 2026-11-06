package cmd_context

// Source: src/cmd_context/extra_cmds/dbg_cmds.cpp, translator_cmd.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteDebugTranslation translates an expression into a fresh AST manager and
// prints both versions before releasing the translated manager.
func (c *Context) WriteDebugTranslation(out io.Writer, expression ast.ASTHandle) (result error) {
	from, err := c.Manager()
	if err != nil {
		return err
	}
	proofMode, err := from.ProofMode()
	if err != nil {
		return err
	}
	to, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: from.Symbols, Numerals: from.NumeralService(), Budget: from.Budget(), ProofMode: proofMode, Params: c.config.Params, Globals: c.config.Globals})
	if to != nil {
		defer func() { result = errors.Join(result, to.Close()) }()
	}
	if err != nil {
		return err
	}
	translation, err := ast.NewASTTranslation(from, to, true)
	if translation != nil {
		defer func() { result = errors.Join(result, translation.Close()) }()
	}
	if err != nil {
		return err
	}
	translated, err := translation.Translate(expression)
	if err != nil {
		return err
	}
	if err = c.WriteExpression(out, expression); err != nil {
		return err
	}
	if _, err = fmt.Fprint(out, "\n--->\n"); err != nil {
		return err
	}
	format, err := to.SMT2PPExprTree(translated, nil)
	if err != nil {
		return err
	}
	if err = c.writeASTFormatWithManager(out, to, format); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}
