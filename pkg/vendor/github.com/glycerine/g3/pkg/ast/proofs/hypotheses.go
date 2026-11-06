package proofs

// Source: src/ast/proofs/proof_utils.cpp:394-421;
// src/util/container_util.h:48-52.

import "github.com/glycerine/g3/pkg/ast"

// hypothesisSets tracks the borrowed expression hypotheses of each visited
// proof. It is the concrete expr_set use in reduce_hypotheses0: the first
// parent's set is shared until a second nonempty parent requires a union.
type hypothesisSets struct {
	manager *ast.ASTManager
	byProof map[ast.ASTHandle]map[ast.ASTHandle]struct{}
	owned   []*hypothesisSetOwner
}

func newHypothesisSets(m *ast.ASTManager) *hypothesisSets {
	return &hypothesisSets{manager: m, byProof: make(map[ast.ASTHandle]map[ast.ASTHandle]struct{})}
}

func (h *hypothesisSets) add(p ast.ASTHandle) error {
	if h == nil || h.manager == nil {
		return ast.ErrASTHandle
	}
	isHypothesis, err := h.manager.IsProofKind(p, ast.ASTPrHypothesis)
	if err != nil {
		return err
	}
	var result map[ast.ASTHandle]struct{}
	if isHypothesis {
		fact, e := h.manager.GetProofFact(p)
		if e != nil {
			return e
		}
		owner, e := h.newOwnedSet(398, nil)
		if e != nil {
			return e
		}
		owner.values[fact] = struct{}{}
		result = owner.values
	} else {
		n, e := h.manager.GetProofNumParents(p)
		if e != nil {
			return e
		}
		inherited := false
		for i := uint32(0); i < n; i++ {
			parent, e := h.manager.GetProofParent(p, i)
			if e != nil {
				return e
			}
			other := h.byProof[parent]
			if other == nil {
				continue
			}
			if result == nil {
				result = other
				inherited = true
				continue
			}
			if inherited {
				owner, e := h.newOwnedSet(412, result)
				if e != nil {
					return e
				}
				result = owner.values
				inherited = false
			}
			for expr := range other {
				result[expr] = struct{}{}
			}
		}
	}
	h.byProof[p] = result
	return nil
}

func (h *hypothesisSets) reset() {
	if h != nil {
		for _, owner := range h.owned {
			h.releaseOwnedSet(owner, "src/util/util.h", 224)
		}
		h.owned = nil
		clear(h.byProof)
	}
}
