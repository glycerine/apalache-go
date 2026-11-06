package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (i *forwardTrigInvocation) newEntry(term, fresh ast.ASTHandle) (_ *forwardTrigEntry, err error) {
	m := i.owner.manager
	e := &forwardTrigEntry{}
	if e.term, err = m.NewRef(term); err != nil {
		return nil, err
	}
	if e.fresh, err = m.NewRef(fresh); err != nil {
		return nil, errors.Join(err, m.DelRef(&e.term))
	}
	if i.proofs {
		eq, x := m.MkEq(fresh, term)
		if x != nil {
			return nil, x
		}
		intro, x := m.MkProofDefIntro(eq)
		if x != nil {
			return nil, x
		}
		apply, x := m.MkProofApplyDef(term, fresh, intro)
		if x != nil {
			return nil, x
		}
		if e.proof, err = m.NewRef(apply); err != nil {
			return nil, err
		}
	}
	i.cache[term] = e
	i.entries = append(i.entries, e)
	return e, nil
}
func (i *forwardTrigInvocation) addConstraint(h ast.ASTHandle, e *forwardTrigEntry) error {
	m := i.owner.manager
	r, x := m.NewRef(h)
	if x != nil {
		return x
	}
	i.constraints = append(i.constraints, r)
	if !i.proofs {
		return nil
	}
	p, x := m.RefGet(e.proof)
	if x != nil {
		return x
	}
	lemma, x := m.MkProofTheoryLemma(i.owner.arithmetic, h, []ast.ASTHandle{p}, nil)
	if x != nil {
		return x
	}
	pr, x := m.NewRef(lemma)
	if x != nil {
		return x
	}
	i.proofRefs = append(i.proofRefs, pr)
	return nil
}
