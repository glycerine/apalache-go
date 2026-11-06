package core

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (t *PBPreprocessTactic) ownedRational(q *big.Rat) (unumeral.OwnedRational, error) {
	v, e := t.manager.NumeralService().NewValue()
	if e != nil {
		return v, e
	}
	if e = t.manager.NumeralService().SetString(&v, q.RatString()); e != nil {
		return v, errors.Join(e, t.manager.NumeralService().Del(&v))
	}
	return v, nil
}

func (t *PBPreprocessTactic) makeGE(terms []pbPreprocessTerm, bound *big.Rat) (_ ast.ASTHandle, result error) {
	coefficients := make([]unumeral.OwnedRational, len(terms))
	args := make([]ast.ASTHandle, len(terms))
	created := 0
	defer func() {
		for i := created - 1; i >= 0; i-- {
			result = errors.Join(result, t.manager.NumeralService().Del(&coefficients[i]))
		}
	}()
	for i, term := range terms {
		var e error
		coefficients[i], e = t.ownedRational(term.coefficient)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		created++
		args[i] = term.expression
	}
	k, e := t.ownedRational(bound)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	defer func() { result = errors.Join(result, t.manager.NumeralService().Del(&k)) }()
	return t.manager.MkPseudoBooleanGE(coefficients, args, &k)
}

func (t *PBPreprocessTactic) negateLiteral(h ast.ASTHandle) (ast.ASTHandle, error) {
	if child, yes, e := t.negated(h); e != nil {
		return ast.ASTHandle{}, e
	} else if yes {
		return child, nil
	}
	return t.manager.MkNot(h)
}

func (t *PBPreprocessTactic) normalize(g *tactic.Goal) error {
	n, e := g.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		h, x := g.Form(i)
		if x != nil {
			return x
		}
		child, negated, x := t.negated(h)
		if x != nil {
			return x
		}
		if !negated {
			continue
		}
		ge, x := t.manager.IsPseudoBooleanGE(child)
		if x != nil {
			return x
		}
		if !ge {
			continue
		}
		parsed, x := t.parse(child)
		if x != nil {
			return x
		}
		sum := new(big.Rat)
		terms := make([]pbPreprocessTerm, len(parsed.terms))
		for j, term := range parsed.terms {
			sum.Add(sum, term.coefficient)
			terms[j].coefficient = new(big.Rat).Set(term.coefficient)
			terms[j].expression, x = t.negateLiteral(term.expression)
			if x != nil {
				return x
			}
		}
		bound := new(big.Rat).Sub(sum, parsed.bound)
		bound.Add(bound, big.NewRat(1, 1))
		next, x := t.makeGE(terms, bound)
		if x != nil {
			return x
		}
		pr, x := g.Proof(i)
		if x != nil {
			return x
		}
		dep, x := g.Dependency(i)
		if x != nil {
			return x
		}
		if x = g.Update(i, next, pr, dep); x != nil {
			return x
		}
	}
	return nil
}
