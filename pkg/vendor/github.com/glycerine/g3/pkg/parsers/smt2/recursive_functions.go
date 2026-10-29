package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_define_fun_rec,
// parse_define_funs_rec and parse_rec_fun_decl/body.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type recursiveDeclaration struct {
	name        string
	names       []usymbol.Symbol
	domain      []ast.ASTHandle
	rangeSort   ast.ASTHandle
	declaration ast.ASTHandle
}

func (p *commandParser) recursiveHeader() (d recursiveDeclaration, err error) {
	if _, err = p.astManager(); err != nil {
		return d, err
	}
	if d.name, err = p.symbol(); err != nil {
		return d, err
	}
	if d.names, d.domain, err = p.sortedVariables(); err != nil {
		return d, err
	}
	d.rangeSort, err = p.sort()
	return d, err
}

func (p *commandParser) recursiveBody(d recursiveDeclaration) (result error) {
	oldBindings, oldExprs := p.bindings, len(p.locals.refs)
	var err error
	p.bindings, err = p.env.bindSortedVariables(p.manager, d.names, d.domain, p.bindings, &p.locals)
	if err != nil {
		return err
	}
	defer func() {
		p.bindings = oldBindings
		result = errors.Join(result, p.env.endScopeWithOwningCleanup(), p.locals.shrink(p.manager, oldExprs))
	}()
	body, _, err := p.expression()
	if err != nil {
		return err
	}
	return p.context.DefineRecursiveFunction(d.declaration, d.names, body)
}

func (p *commandParser) defineRecursiveFunctions(mutual bool) error {
	var declarations []recursiveDeclaration
	if mutual {
		if err := p.expect(LeftParenToken); err != nil {
			return err
		}
		for p.token != RightParenToken {
			if err := p.expect(LeftParenToken); err != nil {
				return err
			}
			d, err := p.recursiveHeader()
			if err != nil {
				return err
			}
			declarations = append(declarations, d)
			if err := p.expect(RightParenToken); err != nil {
				return err
			}
		}
		if err := p.expect(RightParenToken); err != nil {
			return err
		}
	} else {
		d, err := p.recursiveHeader()
		if err != nil {
			return err
		}
		declarations = append(declarations, d)
	}
	// Publish every declaration before parsing any body, allowing forward and
	// mutual references. Declarations remain installed if a later body fails.
	for i := range declarations {
		d := &declarations[i]
		var err error
		d.declaration, err = p.context.DeclareFunction(d.name, d.domain, d.rangeSort)
		if err != nil {
			return err
		}
	}
	if mutual {
		if err := p.expect(LeftParenToken); err != nil {
			return err
		}
	}
	for _, d := range declarations {
		if mutual && p.token == RightParenToken {
			return fmt.Errorf("the number of declarations does not match number of supplied definitions")
		}
		if err := p.recursiveBody(d); err != nil {
			return err
		}
	}
	if mutual {
		if err := p.expect(RightParenToken); err != nil {
			return err
		}
	}
	return p.requireEnd()
}
