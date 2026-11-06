package smt2

// Source: src/muz/fp/dl_cmds.cpp dl_query_cmd::set_next_arg.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *commandParser) validateFixedpointQueryDeclaration(declaration ast.ASTHandle) error {
	manager, err := p.astManager()
	if err != nil {
		return err
	}
	family, err := manager.GetFamilyID(declaration)
	if err != nil {
		return err
	}
	if family != ast.NullFamilyID {
		return fmt.Errorf("invalid query argument: expected uninterpreted function name")
	}
	predicates, err := p.context.FixedpointPredicates()
	if err != nil {
		return err
	}
	for _, predicate := range predicates {
		if predicate == declaration {
			return nil
		}
	}
	return fmt.Errorf("invalid query argument: expected a predicate registered as a relation")
}
