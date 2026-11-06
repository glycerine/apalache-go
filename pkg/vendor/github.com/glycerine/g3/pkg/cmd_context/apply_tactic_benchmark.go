package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp:328-343;
// src/cmd_context/cmd_context.cpp:2427-2447.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (c *Context) writeAppliedTacticBenchmark(out io.Writer, goals []*tactic.Goal) (result error) {
	if len(goals) == 0 {
		return ast.ErrASTDomain
	}
	formulas := make([]ast.ASTHandle, 0)
	if len(goals) == 1 {
		var err error
		formulas, err = goals[0].AppendFormulas(formulas)
		if err != nil {
			return err
		}
	} else {
		branches := make([]ast.ASTHandle, 0, len(goals))
		for _, goal := range goals {
			branch, err := goal.AppendFormulas(nil)
			if err != nil {
				return err
			}
			conjunction, err := c.manager.MkAndArray(branch)
			if err != nil {
				return err
			}
			branches = append(branches, conjunction)
		}
		root, err := c.manager.MkOrArray(branches)
		if err != nil {
			return err
		}
		ref, err := c.manager.NewRef(root)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, c.manager.DelRef(&ref)) }()
		formulas = append(formulas, root)
	}
	if c.logic != "" {
		if _, err := fmt.Fprintf(out, "(set-logic %s)\n", c.logic); err != nil {
			return err
		}
	}
	util, err := ast.NewASTPPUtil(c.manager)
	if err != nil {
		return err
	}
	if err = util.CollectAll(formulas); err != nil {
		return err
	}
	if err = util.DisplayDeclarations(out); err != nil {
		return err
	}
	if err = util.DisplayAssertions(out, formulas, true); err != nil {
		return err
	}
	_, err = io.WriteString(out, "(check-sat)\n")
	return err
}
