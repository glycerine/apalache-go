package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp get_subst and process_quantifier.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *qfnraPurifyConfig) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := c.invocation.owner.manager.GetKind(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if kind == ast.ASTQuantifier {
		result, proof, err := c.invocation.processQuantifier(term)
		return result, proof, true, err
	}
	if kind == ast.ASTApp && c.invocation.owner.elimRoot {
		return c.invocation.processRootObject(term)
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}

func (i *qfnraPurifyInvocation) holdQuantifierResult(value ast.ASTHandle) error {
	if value.IsNull() {
		return nil
	}
	ref, err := i.owner.manager.NewRef(value)
	if err == nil {
		i.quantRefs = append(i.quantRefs, ref)
	}
	return err
}

func (i *qfnraPurifyInvocation) processQuantifier(q ast.ASTHandle) (result, proof ast.ASTHandle, err error) {
	m := i.owner.manager
	body, err := m.GetQuantifierBody(q)
	if err != nil {
		return result, proof, err
	}
	child, err := newQFNRAPurifyInvocation(i.owner, i.goal)
	if err != nil {
		return result, proof, err
	}
	child.trig = i.trig
	child.ownsTrig = false
	defer func() { err = errors.Join(err, child.close()) }()
	config := &qfnraPurifyConfig{invocation: child}
	var traversal *rewriter.Traversal
	if i.proofs {
		traversal, err = rewriter.NewProofTraversal(m, config)
	} else {
		traversal, err = rewriter.NewTraversal(m, config)
	}
	if traversal != nil {
		defer func() { err = errors.Join(err, traversal.Close()) }()
	}
	if err != nil {
		return result, proof, err
	}
	bodyRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, proof, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&bodyRef)) }()
	proofRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, proof, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&proofRef)) }()
	if i.proofs {
		err = traversal.RewriteWithProof(body, bodyRef, proofRef)
	} else {
		err = traversal.Rewrite(body, bodyRef)
	}
	if err != nil {
		return result, proof, err
	}
	rewritten, err := m.RefGet(bodyRef)
	if err != nil {
		return result, proof, err
	}
	bodyProof, err := m.RefGet(proofRef)
	if err != nil {
		return result, proof, err
	}
	inner := rewritten
	var boundEntries []*qfnraPurifyEntry
	for _, entry := range child.entries {
		if entry.bind {
			boundEntries = append(boundEntries, entry)
		}
	}
	if err = i.owner.traceQuantifier(body, rewritten, len(boundEntries)); err != nil {
		return result, proof, err
	}
	if len(child.constraints) != 0 || len(boundEntries) != 0 {
		terms := make([]ast.ASTHandle, 0, len(child.constraints)+1)
		for _, ref := range child.constraints {
			value, e := m.RefGet(ref)
			if e != nil {
				return result, proof, e
			}
			terms = append(terms, value)
		}
		terms = append(terms, rewritten)
		conjunction, e := m.MkAndArgs(terms)
		if e != nil {
			return result, proof, e
		}
		conjunctionRef, e := m.NewRef(conjunction)
		if e != nil {
			return result, proof, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&conjunctionRef)) }()
		inner = conjunction
		if len(boundEntries) != 0 {
			shiftedRef, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return result, proof, e
			}
			defer func() { err = errors.Join(err, m.DelRef(&shiftedRef)) }()
			shifter, e := rewriter.NewVarShifter(m)
			if shifter != nil {
				defer func() { err = errors.Join(err, shifter.Close()) }()
			}
			if e != nil {
				return result, proof, e
			}
			if e = shifter.ShiftBy(conjunction, uint32(len(boundEntries)), shiftedRef); e != nil {
				return result, proof, e
			}
			shifted, e := m.RefGet(shiftedRef)
			if e != nil {
				return result, proof, e
			}
			bound := make([]ast.ASTHandle, 0, len(boundEntries))
			sorts := make([]ast.ASTHandle, 0, len(boundEntries))
			names := make([]usymbol.Symbol, 0, len(boundEntries))
			prefix := "x"
			for _, entry := range boundEntries {
				fresh, e := m.RefGet(entry.fresh)
				if e != nil {
					return result, proof, e
				}
				sort, e := m.GetSort(fresh)
				if e != nil {
					return result, proof, e
				}
				name, e := m.MkFreshVarName(&prefix)
				if e != nil {
					return result, proof, e
				}
				bound, sorts, names = append(bound, fresh), append(sorts, sort), append(names, name)
			}
			abstractor, e := ast.NewExprAbstractor(m)
			if e != nil {
				return result, proof, e
			}
			defer func() { err = errors.Join(err, abstractor.Close()) }()
			if e = abstractor.Abstract(0, bound, shifted, bodyRef); e != nil {
				return result, proof, e
			}
			abstracted, e := m.RefGet(bodyRef)
			if e != nil {
				return result, proof, e
			}
			inner, e = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: abstracted})
			if e != nil {
				return result, proof, e
			}
			if e = m.SetRef(bodyRef, inner); e != nil {
				return result, proof, e
			}
		}
		if i.proofs {
			premises := make([]ast.ASTHandle, 0, len(child.proofRefs))
			for _, ref := range child.proofRefs {
				p, e := m.RefGet(ref)
				if e != nil {
					return result, proof, e
				}
				premises = append(premises, p)
			}
			bodyProof, e = m.MkProofRewriteStar(body, inner, premises)
			if e != nil {
				return result, proof, e
			}
			if e = m.SetRef(proofRef, bodyProof); e != nil {
				return result, proof, e
			}
		}
	}
	result, err = m.UpdateQuantifierBody(q, inner)
	if err != nil {
		return result, proof, err
	}
	if i.proofs {
		if bodyProof.IsNull() {
			bodyProof, err = m.MkProofReflexivity(body)
			if err != nil {
				return result, proof, err
			}
		}
		proof, err = m.MkProofQuantIntro(q, result, bodyProof)
		if err != nil {
			return result, proof, err
		}
	}
	if err = i.holdQuantifierResult(result); err == nil {
		err = i.holdQuantifierResult(proof)
	}
	return result, proof, err
}
