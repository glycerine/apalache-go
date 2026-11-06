package api

// Source: src/api/api_goal.cpp:167-181.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"strings"
)

func (g *APIGoal) String() (string, error) {
	if err := g.open(); err != nil {
		return "", err
	}
	return apiGoalString(g.owner, g.manager, g.goal)
}

func apiGoalString(owner *smtContextState, manager *ast.ASTManager, goal *tactic.Goal) (string, error) {
	var output strings.Builder
	printer := &APIASTPrinter{owner: owner, mode: APIASTPrintSMT2}
	output.WriteString("(goal")
	size, err := goal.Size()
	if err != nil {
		return "", err
	}
	for i := uint32(0); i < size; i++ {
		formula, err := goal.Form(i)
		if err != nil {
			return "", err
		}
		body, err := printer.smt2StringIndented(manager, formula, 2)
		if err != nil {
			return "", err
		}
		output.WriteString("\n  ")
		output.WriteString(body)
	}
	output.WriteByte(')')
	return output.String(), nil
}
