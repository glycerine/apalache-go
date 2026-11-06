package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type forwardTrigConfig struct {
	rewriter.DefaultConfiguration
	invocation *forwardTrigInvocation
}

func (c *forwardTrigConfig) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	i := c.invocation
	m := i.owner.manager
	fid, e := m.GetFamilyID(decl)
	if e != nil || fid != i.owner.arithmetic {
		return rewriter.RewriteFailed, e
	}
	op, e := m.GetDeclKind(decl)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if op != forwardTrigSin && op != forwardTrigCos {
		return rewriter.RewriteFailed, nil
	}
	if args.Count() != 1 {
		return rewriter.RewriteFailed, ast.ErrASTDomain
	}
	theta, e := args.At(0)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	term, e := m.MkApp(decl, []ast.ASTHandle{theta})
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if cached := i.cache[term]; cached != nil {
		fresh, x := m.RefGet(cached.fresh)
		if x != nil {
			return rewriter.RewriteFailed, x
		}
		if x = m.SetRef(out, fresh); x != nil {
			return rewriter.RewriteFailed, x
		}
		if i.proofs {
			p, x := m.RefGet(cached.proof)
			if x != nil {
				return rewriter.RewriteFailed, x
			}
			if x = m.SetRef(proof, p); x != nil {
				return rewriter.RewriteFailed, x
			}
		}
		return rewriter.RewriteDone, nil
	}
	pair, ok, e := i.basis(theta)
	if e != nil || !ok {
		return rewriter.RewriteFailed, e
	}
	freshRef := pair.sin
	if op == forwardTrigCos {
		freshRef = pair.cos
	}
	fresh, e := m.RefGet(freshRef)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	entry, e := i.newEntry(term, fresh)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if e = m.SetRef(out, fresh); e != nil {
		return rewriter.RewriteFailed, e
	}
	if i.proofs {
		p, x := m.RefGet(entry.proof)
		if x != nil {
			return rewriter.RewriteFailed, x
		}
		if x = m.SetRef(proof, p); x != nil {
			return rewriter.RewriteFailed, x
		}
	}
	if e = i.unitCircle(pair, entry); e != nil {
		return rewriter.RewriteFailed, e
	}
	return rewriter.RewriteDone, nil
}
