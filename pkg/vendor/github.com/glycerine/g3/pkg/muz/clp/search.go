package clp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *Engine) search(goals []ast.ASTHandle, depth uint32, index int) (status lbool.LBool, resultErr error) {
	if index == len(goals) {
		return lbool.True, nil
	}
	if depth == 0 {
		return lbool.Undef, nil
	}
	if err := e.verboseSearch(depth, index); err != nil {
		return lbool.Undef, err
	}
	decl, err := e.manager.GetDecl(goals[index])
	if err != nil {
		return lbool.Undef, err
	}
	rules, err := sortedRules(e.rules, decl)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeRules(rules)) }()
	status = lbool.False
	for _, rule := range rules {
		candidate, x := e.tryScopedRule(rule, goals, depth, index)
		if x != nil {
			return lbool.Undef, x
		}
		switch candidate {
		case lbool.True:
			return candidate, nil
		case lbool.Undef:
			status = lbool.Undef
		}
	}
	return status, nil
}
