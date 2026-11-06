package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp process_irrat.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (i *qfnraPurifyInvocation) processRootObject(term ast.ASTHandle) (fresh, proof ast.ASTHandle, matched bool, err error) {
	m := i.owner.manager
	var value polynomial.Algebraic
	matched, err = m.ArithmeticAlgebraicNumeralValue(term, &value)
	if err != nil || !matched {
		return fresh, proof, matched, err
	}
	manager, err := m.ArithmeticAlgebraicManager()
	if err != nil {
		return fresh, proof, true, err
	}
	defer func() { err = errors.Join(err, manager.Del(&value)) }()
	entry := i.cache[term]
	if entry == nil {
		entry, err = i.newEntry(term)
		if err != nil {
			return fresh, proof, true, err
		}
		fresh, err = m.RefGet(entry.fresh)
		if err != nil {
			return fresh, proof, true, err
		}
		err = processRootObjectConstraints(m, i.owner.arithmetic, &value, fresh, func(h ast.ASTHandle) error {
			return i.addConstraint(h, entry)
		})
		if err != nil {
			return fresh, proof, true, err
		}
	} else {
		fresh, err = m.RefGet(entry.fresh)
		if err != nil {
			return fresh, proof, true, err
		}
	}
	if i.proofs {
		proof, err = m.RefGet(entry.proof)
	}
	return fresh, proof, true, err
}
