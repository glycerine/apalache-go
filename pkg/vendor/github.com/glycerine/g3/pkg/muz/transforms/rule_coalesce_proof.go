package transforms

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// A merged clause needs both source clauses. The source unary hyper-resolution
// placeholder discards one branch and does not justify its disjunctive guard.
// Use ordinary modus ponens and a rewrite obligation for the merge implication.
func makeCoalesceProof(m *ast.ASTManager, source, target, merged *base.Rule) (result ast.ASTRef, resultErr error) {
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	// Allocate owning slots before constructing the proof chain. Each later
	// constructor can fail, and a completed intermediate proof already owns its
	// source clauses even before the final proof is published to the caller.
	var temporary [7]ast.ASTRef
	defer func() {
		for i := len(temporary) - 1; i >= 0; i-- {
			if temporary[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&temporary[i]))
			}
		}
	}()
	for i := range temporary {
		temporary[i], err = m.NewRef(ast.ASTHandle{})
		if err != nil {
			return result, err
		}
	}
	var parents, facts []ast.ASTHandle
	for _, r := range []*base.Rule{source, target} {
		p, found, e := r.Proof()
		if e != nil {
			return result, e
		}
		if !found {
			return result, errors.New("muz/transforms: coalesced source rule has no proof")
		}
		fact, e := m.GetProofFact(p)
		if e != nil {
			return result, e
		}
		parents = append(parents, p)
		facts = append(facts, fact)
	}
	formula, err := merged.ToFormula()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&formula)) }()
	conclusion, err := m.RefGet(formula)
	if err != nil {
		return result, err
	}
	inner, err := m.MkImplies(facts[1], conclusion)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[0], inner); err != nil {
		return result, err
	}
	theorem, err := m.MkImplies(facts[0], inner)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[1], theorem); err != nil {
		return result, err
	}
	truth, err := m.MkTrue()
	if err != nil {
		return result, err
	}
	rewrite, err := m.MkProofRewrite(truth, theorem)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[2], rewrite); err != nil {
		return result, err
	}
	trueProof, err := m.MkProofTrue()
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[3], trueProof); err != nil {
		return result, err
	}
	proof, err := m.MkProofModusPonens(trueProof, rewrite)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[4], proof); err != nil {
		return result, err
	}
	proof, err = m.MkProofModusPonens(parents[0], proof)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[5], proof); err != nil {
		return result, err
	}
	proof, err = m.MkProofModusPonens(parents[1], proof)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(temporary[6], proof); err != nil {
		return result, err
	}
	return m.NewRef(proof)
}

// matchCoalesceProof recognizes only the complete two-clause construction above.
func matchCoalesceProof(m *ast.ASTManager, node ast.ASTHandle) (source, target ast.ASTHandle, matched bool, err error) {
	var chain [3]ast.ASTHandle
	chain[0] = node
	for i := 0; i < 3; i++ {
		yes, e := m.IsProofKind(chain[i], ast.ASTPrModusPonens)
		if e != nil || !yes {
			return source, target, false, e
		}
		a, e := m.GetProofParent(chain[i], 0)
		if e != nil {
			return source, target, false, e
		}
		b, e := m.GetProofParent(chain[i], 1)
		if e != nil {
			return source, target, false, e
		}
		if i == 0 {
			target = a
			chain[1] = b
		} else if i == 1 {
			source = a
			chain[2] = b
		} else {
			yes, e = m.IsProofKind(a, ast.ASTPrTrue)
			if e != nil || !yes {
				return source, target, false, e
			}
			var left, right ast.ASTHandle
			yes, e = m.MatchProofRewrite(b, &left, &right)
			if e != nil || !yes || !m.IsTrue(left) {
				return source, target, false, e
			}
			sf, e := m.GetProofFact(source)
			if e != nil {
				return source, target, false, e
			}
			tf, e := m.GetProofFact(target)
			if e != nil {
				return source, target, false, e
			}
			conclusion, e := m.GetProofFact(node)
			if e != nil {
				return source, target, false, e
			}
			inner, e := m.MkImplies(tf, conclusion)
			if e != nil {
				return source, target, false, e
			}
			theorem, e := m.MkImplies(sf, inner)
			if e != nil {
				return source, target, false, e
			}
			return source, target, theorem == right, nil
		}
	}
	return source, target, false, nil
}
