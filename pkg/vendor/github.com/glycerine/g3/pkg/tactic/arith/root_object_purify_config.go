package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

type rootObjectConfig struct {
	rewriter.DefaultConfiguration
	invocation *rootObjectInvocation
}

func (c *rootObjectConfig) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	i := c.invocation
	if !i.owner.elim {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	var value polynomial.Algebraic
	matched, e := i.owner.manager.ArithmeticAlgebraicNumeralValue(term, &value)
	if e != nil || !matched {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	am, e := i.owner.manager.ArithmeticAlgebraicManager()
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	defer am.Del(&value)
	entry := i.cache[term]
	created := entry == nil
	if created {
		entry, e = i.entry(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
	}
	fresh, e := i.owner.manager.RefGet(entry.fresh)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if created {
		if e = i.process(&value, fresh, entry); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
	}
	proof := ast.ASTHandle{}
	if i.proofs {
		proof, e = i.owner.manager.RefGet(entry.proof)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
	}
	return fresh, proof, true, nil
}
