package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp convert_basis and process_sin_cos.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// The source shares the angle basis across per-quantifier rewriter invocations.
type qfnraForwardTrigState struct {
	owner *ForwardTrigPurifyTactic
	inv   *forwardTrigInvocation
}

func (s *qfnraForwardTrigState) prepare(i *qfnraPurifyInvocation) error {
	m := i.owner.manager
	found, err := visitArithmeticGoal(i.goal, func(term ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return false, nil
		}
		family, e := m.GetFamilyID(term)
		if e != nil || family != i.owner.arithmetic {
			return false, e
		}
		op, e := m.GetDeclKind(term)
		return op == forwardTrigSin || op == forwardTrigCos, e
	})
	if err != nil || !found {
		return err
	}
	return s.ensure(i)
}

func (s *qfnraForwardTrigState) ensure(i *qfnraPurifyInvocation) (err error) {
	if s.inv != nil {
		return nil
	}
	t := i.owner
	owner, err := NewForwardTrigPurifyTactic(t.manager, t.context, t.globals, &t.params, t.services)
	if err != nil {
		return err
	}
	inv, err := newForwardTrigInvocation(owner, i.goal)
	if err != nil {
		return errors.Join(err, owner.Close())
	}
	if err = inv.findUnsafe(); err != nil {
		return errors.Join(err, inv.close(), owner.Close())
	}
	s.owner, s.inv = owner, inv
	return nil
}

func (s *qfnraForwardTrigState) close() error {
	if s.inv == nil {
		return nil
	}
	err := errors.Join(s.inv.close(), s.owner.Close())
	s.inv, s.owner = nil, nil
	return err
}

func (c *qfnraPurifyConfig) reduceForwardTrig(decl ast.ASTHandle, op int32, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	i := c.invocation
	m := i.owner.manager
	if args.Count() != 1 {
		return rewriter.RewriteFailed, ast.ErrASTDomain
	}
	theta, err := args.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	term, err := m.MkApp(decl, []ast.ASTHandle{theta})
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	entry := i.cache[term]
	if entry == nil {
		if err = i.trig.ensure(i); err != nil {
			return rewriter.RewriteFailed, err
		}
		pair, ok, e := i.trig.inv.basis(theta)
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
		entry, e = i.newEntryWithFresh(term, fresh, false)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		e = forwardTrigUnitCircle(m, pair, func(h ast.ASTHandle) error { return i.addConstraint(h, entry) })
		if e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	fresh, err := m.RefGet(entry.fresh)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = m.SetRef(out, fresh); err != nil {
		return rewriter.RewriteFailed, err
	}
	if i.proofs {
		p, e := m.RefGet(entry.proof)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(proof, p); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	return rewriter.RewriteDone, nil
}

func (i *qfnraPurifyInvocation) publishModel(out *tactic.TacticResult) (err error) {
	if !i.models {
		return nil
	}
	var pairs []*forwardTrigPair
	if i.trig != nil && i.trig.inv != nil {
		pairs = i.trig.inv.pairOrder
	}
	m := i.owner.manager
	filter, err := tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = filter.IncRef(); err != nil {
		return errors.Join(err, filter.Close())
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	for _, decl := range i.freshDecls {
		if err = filter.Insert(decl); err != nil {
			return err
		}
	}
	if len(pairs) == 0 {
		return out.SetModelConverter(filter)
	}
	extension, err := tactic.NewExtensionModel(m, i.owner.services)
	if err != nil {
		return err
	}
	if err = extension.IncRef(); err != nil {
		return errors.Join(err, extension.Close())
	}
	defer func() { err = errors.Join(err, extension.DecRef()) }()
	for _, pair := range pairs {
		x, e := m.RefGet(pair.sin)
		if e != nil {
			return e
		}
		y, e := m.RefGet(pair.cos)
		if e != nil {
			return e
		}
		for _, fresh := range []ast.ASTHandle{x, y} {
			decl, e := m.GetDecl(fresh)
			if e != nil {
				return e
			}
			if e = filter.Insert(decl); e != nil {
				return e
			}
		}
		definition, e := i.trig.inv.thetaDefinition(x, y)
		if e != nil {
			return e
		}
		decl, e := m.GetDecl(pair.theta)
		if e != nil {
			return e
		}
		if e = extension.Insert(decl, definition); e != nil {
			return e
		}
	}
	combined, err := tactic.ConcatModels(m.Budget(), filter, extension)
	if err != nil {
		return err
	}
	if err = out.SetModelConverter(combined); err != nil {
		return errors.Join(err, combined.Close())
	}
	return nil
}
