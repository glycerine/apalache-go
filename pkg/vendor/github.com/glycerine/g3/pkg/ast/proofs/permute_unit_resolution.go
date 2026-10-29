package proofs

// Source: src/ast/proofs/proof_utils.cpp:823-916.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type permuteUnitResolutionState struct {
	manager *ast.ASTManager
	cache   map[ast.ASTHandle]ast.ASTHandle
	pins    []ast.ASTRef
}

func (s *permuteUnitResolutionState) close() (err error) {
	for i := len(s.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, s.manager.DelRef(&s.pins[i]))
	}
	return err
}

func (s *permuteUnitResolutionState) pin(h ast.ASTHandle) error {
	r, err := s.manager.NewRef(h)
	if err == nil {
		s.pins = append(s.pins, r)
	}
	return err
}

func (s *permuteUnitResolutionState) transform(p ast.ASTHandle) (result ast.ASTHandle, err error) {
	if result, found := s.cache[p]; found {
		return result, nil
	}
	n, err := s.manager.GetProofNumParents(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parents := make([]ast.ASTHandle, n)
	for i := range parents {
		parent, e := s.manager.GetProofParent(p, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		parents[i], e = s.transform(parent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	unit, err := s.manager.IsProofKind(p, ast.ASTPrUnitResolution)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	theory := false
	if unit && len(parents) != 0 {
		theory, err = s.manager.IsProofKind(parents[0], ast.ASTPrTheoryLemma)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if theory {
		th := parents[0]
		thCount, e := s.manager.GetProofNumParents(th)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		premises := make([]ast.ASTHandle, 0, int(thCount)+len(parents)-1)
		for i := uint32(0); i < thCount; i++ {
			parent, e := s.manager.GetProofParent(th, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			premises = append(premises, parent)
		}
		premises = append(premises, parents[1:]...)
		decl, e := s.manager.GetDecl(th)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		params, e := s.manager.GetDeclParametersCopy(decl)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		defer func() {
			for i := range params {
				err = errors.Join(err, s.manager.DelParameter(&params[i]))
			}
		}()
		if len(params) == 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		name, e := s.manager.ParameterSymbol(params[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		tid, e := s.manager.FamilyIDForSymbol(name)
		if e != nil || tid == ast.NullFamilyID {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return ast.ASTHandle{}, e
		}
		fact, e := s.manager.GetProofFact(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, err = s.manager.MkProofTheoryLemma(tid, fact, premises, params[1:])
	} else {
		args := append([]ast.ASTHandle(nil), parents...)
		hasFact, e := s.manager.ProofHasFact(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if hasFact {
			fact, e := s.manager.GetProofFact(p)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args = append(args, fact)
		}
		decl, e := s.manager.GetDecl(p)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, err = s.manager.MkApp(decl, args)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	s.cache[p] = result
	return result, nil
}

// PermuteUnitResolution moves unit-resolution premises into an immediately
// preceding theory lemma and publishes the transformed proof.
func PermuteUnitResolution(m *ast.ASTManager, proof ast.ASTRef) (err error) {
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
	state := &permuteUnitResolutionState{manager: m, cache: make(map[ast.ASTHandle]ast.ASTHandle)}
	defer func() { err = errors.Join(err, state.close()) }()
	result, err := state.transform(root)
	if err != nil {
		return err
	}
	return m.SetRef(proof, result)
}
