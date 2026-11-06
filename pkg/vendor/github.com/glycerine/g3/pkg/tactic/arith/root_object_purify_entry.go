package arith

import "github.com/glycerine/g3/pkg/ast"

func (i *rootObjectInvocation) entry(term ast.ASTHandle) (_ *rootObjectEntry, err error) {
	if e := i.cache[term]; e != nil {
		return e, nil
	}
	m := i.owner.manager
	e := &rootObjectEntry{}
	if e.term, err = m.NewRef(term); err != nil {
		return nil, err
	}
	sort, x := m.GetSort(term)
	if x != nil {
		return nil, x
	}
	fresh, x := m.MkFreshConstNullable(nil, sort)
	if x != nil {
		return nil, x
	}
	if e.fresh, err = m.NewRef(fresh); err != nil {
		return nil, err
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
	decl, err := m.GetDecl(fresh)
	if err != nil {
		return nil, err
	}
	i.freshDecls = append(i.freshDecls, decl)
	return e, nil
}
func (i *rootObjectInvocation) add(h ast.ASTHandle, e *rootObjectEntry) error {
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
