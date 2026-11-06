package proofs

// Source: src/ast/proofs/proof_utils.cpp:918-1006.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type pushInstantiationsState struct {
	manager *ast.ASTManager
	subst   *rewriter.VarSubstitution
	tmp     ast.ASTRef
	pins    []ast.ASTRef
}

func newPushInstantiationsState(m *ast.ASTManager) (*pushInstantiationsState, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	subst, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if subst != nil {
			_ = subst.Close()
		}
		return nil, err
	}
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, subst.Close())
	}
	return &pushInstantiationsState{manager: m, subst: subst, tmp: tmp}, nil
}

func (s *pushInstantiationsState) close() (err error) {
	for i := len(s.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, s.manager.DelRef(&s.pins[i]))
	}
	err = errors.Join(err, s.manager.DelRef(&s.tmp))
	err = errors.Join(err, s.subst.Close())
	return err
}

func (s *pushInstantiationsState) pin(h ast.ASTHandle) error {
	r, err := s.manager.NewRef(h)
	if err == nil {
		s.pins = append(s.pins, r)
	}
	return err
}

func (s *pushInstantiationsState) substitute(term ast.ASTHandle, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := s.subst.Substitute(term, values, s.tmp); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := s.manager.RefGet(s.tmp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.pin(h); err != nil {
		return ast.ASTHandle{}, err
	}
	return h, nil
}

func (s *pushInstantiationsState) compose(values, outer []ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), values...)
	for i := range result {
		h, err := s.substitute(result[i], outer)
		if err != nil {
			return nil, err
		}
		result[i] = h
	}
	return result, nil
}

func (s *pushInstantiationsState) instantiate(values []ast.ASTHandle, formula ast.ASTHandle) (ast.ASTHandle, error) {
	if len(values) == 0 {
		return formula, nil
	}
	kind, err := s.manager.GetKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.ASTQuantifier {
		return formula, nil
	}
	forall, err := s.manager.IsForall(formula)
	if err != nil || !forall {
		return formula, err
	}
	n, err := s.manager.GetNumDeclarations(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(n) != uint64(len(values)) {
		// The source leaves the quantified formula unchanged after its trace.
		return formula, s.manager.TraceProofUtilsQuantifierMismatch(formula, uint32(len(values)))
	}
	body, err := s.manager.GetQuantifierBody(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.substitute(body, values)
}

func (s *pushInstantiationsState) push(p ast.ASTHandle, outer []ast.ASTHandle) (_ ast.ASTHandle, err error) {
	conclusionRef, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, s.manager.DelRef(&conclusionRef)) }()
	out, err := ast.NewASTProofHyperOutput(s.manager)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	var premises []ast.ASTHandle
	var ownedPremises []ast.ASTHandle
	defer func() {
		for i := len(ownedPremises) - 1; i >= 0; i-- {
			err = errors.Join(err, s.manager.DecRef(ownedPremises[i]))
		}
	}()
	matched, err := s.manager.MatchProofHyperResolve(p, &premises, conclusionRef, out)
	ownedPremises = append(ownedPremises, premises...)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if matched {
		positions := make([]ast.ASTProofPosition, out.PositionsLen())
		for i := range positions {
			positions[i], err = out.Position(uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		substitutions := make([][]ast.ASTHandle, out.SubstitutionsLen())
		for i := range substitutions {
			borrowed, e := out.Substitution(uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			substitutions[i] = append([]ast.ASTHandle(nil), borrowed...)
		}
		if len(substitutions) != len(premises) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		for i := range premises {
			substitutions[i], err = s.compose(substitutions[i], outer)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			premises[i], err = s.push(premises[i], substitutions[i])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			substitutions[i] = nil
		}
		conclusion, e := s.manager.RefGet(conclusionRef)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		conclusion, err = s.instantiate(outer, conclusion)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result, e := s.manager.MkProofHyperResolve(premises, conclusion, positions, substitutions)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return result, s.pin(result)
	}
	if len(outer) == 0 {
		return p, nil
	}
	modus, err := s.manager.IsProofKind(p, ast.ASTPrModusPonens)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if modus {
		n, e := s.manager.GetProofNumParents(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if n != 2 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		p0, e := s.manager.GetProofParent(p, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		p1, e := s.manager.GetProofParent(p, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		f0, e := s.manager.GetProofFact(p0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		fact, e := s.manager.GetProofFact(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if f0 == fact {
			return s.push(p0, outer)
		}
		var left, right ast.ASTHandle
		rewrite, e := s.manager.MatchProofRewrite(p1, &left, &right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if rewrite {
			leftKind, e := s.manager.GetKind(left)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			rightKind, e := s.manager.GetKind(right)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if leftKind == ast.ASTQuantifier && rightKind == ast.ASTQuantifier {
				leftCount, e := s.manager.GetNumDeclarations(left)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				rightCount, e := s.manager.GetNumDeclarations(right)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if leftCount == rightCount {
					left, err = s.instantiate(outer, left)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					right, err = s.instantiate(outer, right)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					p1, e = s.manager.MkProofRewrite(left, right)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					if e = s.pin(p1); e != nil {
						return ast.ASTHandle{}, e
					}
					p0, err = s.push(p0, outer)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					result, e := s.manager.MkProofModusPonens(p0, p1)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					return result, s.pin(result)
				}
			}
		}
	}
	fact, err := s.manager.GetProofFact(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := s.instantiate(outer, fact)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := s.manager.MkProofHyperResolve([]ast.ASTHandle{p}, conclusion, nil, [][]ast.ASTHandle{append([]ast.ASTHandle(nil), outer...)})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return result, s.pin(result)
}

// PushInstantiationsUp moves hyper-resolution substitutions toward proof
// leaves and publishes the resulting owning proof reference.
func PushInstantiationsUp(m *ast.ASTManager, proof ast.ASTRef) (err error) {
	if m == nil {
		return ast.ErrASTHandle
	}
	root, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	yes, err := m.IsProof(root)
	if err != nil {
		return err
	}
	if !yes {
		return ast.ErrASTDomain
	}
	state, err := newPushInstantiationsState(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, state.close()) }()
	result, err := state.push(root, nil)
	if err != nil {
		return err
	}
	return m.SetRef(proof, result)
}
