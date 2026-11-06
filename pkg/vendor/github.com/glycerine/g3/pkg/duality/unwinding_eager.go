package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// DoEagerDeduction checks history conjectures conjunct by conjunct in source
// order. Only UNSAT local bound checks publish upper facts. HistoryProposer is
// the concrete proposer instantiated by the live source LearnFrom consumer.
// Source: duality_solver.cpp:1636-1652.
func (u *Unwinding) DoEagerDeduction(node *Node, proposers []*HistoryProposer, checker *LocalEdgeChecker, aux solver.Solver) (result error) {
	if u == nil || node == nil || node.closed || node.Owner != u.graph || node.Outgoing == nil || checker == nil || checker.closed || aux == nil {
		return ErrWrapperOwner
	}
	var facts []*Transformer
	var expressions []*Expr
	defer func() {
		for _, term := range expressions {
			result = errors.Join(result, term.Close())
		}
		for _, fact := range facts {
			result = errors.Join(result, fact.Close())
		}
	}()
	m := u.graph.context.manager
	for _, proposer := range proposers {
		conjectures, err := proposer.Propose(node)
		if err != nil {
			return err
		}
		facts = append(facts, conjectures...)
		for _, conjecture := range conjectures {
			bound, err := conjecture.Clone()
			if err != nil {
				return err
			}
			facts = append(facts, bound)
			formula, err := bound.Formula()
			if err != nil {
				return err
			}
			expressions = append(expressions, formula)
			conjuncts, err := CollectConjuncts(formula, true)
			if err != nil {
				return err
			}
			expressions = append(expressions, conjuncts...)
			for _, conjunct := range conjuncts {
				h, err := conjunct.Raw()
				if err != nil {
					return err
				}
				if err = m.SetRef(bound.formula, h); err != nil {
					return err
				}
				status, err := checker.CheckEdgeCaching(node.Outgoing, bound)
				if err != nil {
					return err
				}
				if status == lbool.False {
					if _, err = u.update(node, bound, aux, true); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
