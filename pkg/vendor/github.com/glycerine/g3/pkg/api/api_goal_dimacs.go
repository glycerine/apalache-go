package api

// Source: src/api/api_goal.cpp:198-214.

import (
	"bytes"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *APIGoal) DIMACS(includeNames bool) (string, error) {
	if err := g.open(); err != nil {
		return "", err
	}
	cnf, err := g.goal.IsCNF()
	if err != nil {
		return "", err
	}
	if !cnf {
		return "", ast.ErrASTDomain
	}
	var output bytes.Buffer
	if err = g.goal.DisplayDIMACSWithNames(&output, includeNames); err != nil {
		return "", err
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
}
