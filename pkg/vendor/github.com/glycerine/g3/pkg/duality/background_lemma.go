package duality

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// proveBackgroundLiteral turns a native refutation of background and not-fact
// into a lemma of fact. Only the negated query assertion becomes a hypothesis;
// the background assertions remain explicit leaves in the exported proof.
// A null reference means that the background does not establish the fact.
func (g *RPFP) proveBackgroundLiteral(factory solver.Factory, background []*Expr, fact ast.ASTHandle) (out ast.ASTRef, result error) {
	m := g.context.manager
	call := &proofWitnessCall{manager: m}
	defer func() {
		if err := call.close(); err != nil {
			g.pendingProofWitnesses = append(g.pendingProofWitnesses, call)
			result = errors.Join(result, err)
		}
		if result != nil && out != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&out))
		}
	}()
	var err error
	call.native, err = factory.New(m, nil, true, false, false, "")
	if err != nil {
		return out, err
	}
	for _, axiom := range background {
		h, err := axiom.Raw()
		if err != nil {
			return out, err
		}
		if err := call.native.Assert(h); err != nil {
			return out, err
		}
	}
	negated, err := call.pin(m.MkNot(fact))
	if err != nil {
		return out, err
	}
	if err := call.native.Assert(negated); err != nil {
		return out, err
	}
	status, err := call.native.CheckSat(nil)
	if err != nil {
		return out, err
	}
	if status == lbool.True {
		return out, fmt.Errorf("quantified premise is not entailed: %w", ErrProofWitness)
	}
	if status != lbool.False {
		reason, _ := call.native.ReasonUnknown()
		return out, fmt.Errorf("quantified premise entailment is unknown (%s): %w", reason, ErrProofWitness)
	}
	proof, err := call.native.Proof()
	if err != nil {
		return out, err
	}
	if proof.IsNull() {
		return out, fmt.Errorf("quantified premise entailment has no proof: %w", ErrProofWitness)
	}
	proof, err = call.pin(proof, nil)
	if err != nil {
		return out, err
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(p ast.ASTHandle) (ast.ASTHandle, error) {
		if h, ok := memo[p]; ok {
			return h, nil
		}
		asserted, err := m.IsProofKind(p, ast.ASTPrAsserted)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if asserted {
			h, err := m.GetProofFact(p)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if h == negated {
				h, err = call.pin(m.MkProofHypothesis(h))
				if err == nil {
					memo[p] = h
				}
				return h, err
			}
		}
		count, err := m.GetProofNumParents(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		n, err := m.GetNumArgs(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, n)
		changed := false
		for i := uint32(0); i < n; i++ {
			args[i], err = m.GetArg(p, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if i < count {
				h, err := visit(args[i])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed = changed || h != args[i]
				args[i] = h
			}
		}
		h := p
		if changed {
			decl, err := m.GetDecl(p)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			h, err = call.pin(m.MkApp(decl, args))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		memo[p] = h
		return h, nil
	}
	contradiction, err := visit(proof)
	if err != nil {
		return out, err
	}
	lemma, err := call.pin(m.MkProofLemma(contradiction, fact))
	if err != nil {
		return out, err
	}
	return m.NewRef(lemma)
}
