package ast

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// Source fixed declaration caches intentionally ignore the caller's requested
// parent count; hyper-resolve caches the first requested count instead.
func astProofDescription(k int32, requested uint32) (name string, parents uint32, array bool) {
	parents = requested
	switch k {
	case ASTPrTrue:
		return "true-axiom", 0, false
	case ASTPrAsserted:
		return "asserted", 0, false
	case ASTPrGoal:
		return "goal", 2, false
	case ASTPrModusPonens:
		return "mp", 2, false
	case ASTPrReflexivity:
		return "refl", 0, false
	case ASTPrSymmetry:
		return "symm", 1, false
	case ASTPrTransitivity:
		return "trans", 2, false
	case ASTPrTransitivityStar:
		return "trans*", parents, true
	case ASTPrMonotonicity:
		return "monotonicity", parents, true
	case ASTPrQuantIntro:
		return "quant-intro", 1, false
	case ASTPrDistributivity:
		return "distributivity", parents, true
	case ASTPrAndElim:
		return "and-elim", 1, false
	case ASTPrNotOrElim:
		return "not-or-elim", 1, false
	case ASTPrRewrite:
		return "rewrite", 0, false
	case ASTPrRewriteStar:
		return "rewrite*", parents, true
	case ASTPrPullQuant:
		return "pull-quant", 0, false
	case ASTPrPushQuant:
		return "push-quant", 0, false
	case ASTPrElimUnusedVars:
		return "elim-unused", 0, false
	case ASTPrDER:
		return "der", 0, false
	case ASTPrQuantInst:
		return "quant-inst", 0, false
	case ASTPrHypothesis:
		return "hypothesis", 0, false
	case ASTPrLemma:
		return "lemma", 1, false
	case ASTPrUnitResolution:
		return "unit-resolution", parents, true
	case ASTPrIffTrue:
		return "iff-true", 1, false
	case ASTPrIffFalse:
		return "iff-false", 1, false
	case ASTPrCommutativity:
		return "commutativity", 0, false
	case ASTPrDefAxiom:
		return "def-axiom", 0, false
	case ASTPrDefIntro:
		return "intro-def", 0, false
	case ASTPrApplyDef:
		return "apply-def", parents, true
	case ASTPrIffOEq:
		return "iff~", 1, false
	case ASTPrNNFPos:
		return "nnf-pos", parents, true
	case ASTPrNNFNeg:
		return "nnf-neg", parents, true
	case ASTPrSkolemize:
		return "sk", 0, false
	case ASTPrModusPonensOEq:
		return "mp~", 2, false
	case ASTPrTheoryLemma:
		return "th-lemma", parents, true
	case ASTPrHyperResolve:
		return "hyper-res", parents, false
	}
	return "", parents, false
}
func (m *ASTManager) newProofDecl(name string, k int32, ps []ASTParameter, parents uint32, pin bool) (astNodeRef, error) {
	// The trailing Bool sort follows all Proof sorts in the source domain.
	var inline [16]ASTHandle
	domain := inline[:0]
	if parents == math.MaxUint32 {
		return astNodeRef{}, arena.ErrBounds
	}
	for i := uint32(0); i <= parents; i++ {
		sort := m.basic.proofSort
		if i == parents {
			sort = m.basic.boolSort
		}
		domain = append(domain, m.external(sort))
	}
	return m.basicNamedDecl(name, k, domain, m.basic.proofSort, ASTFuncFlags{}, ps, pin)
}
func (m *ASTManager) proofDecl(k int32, ps []ASTParameter, parents uint32) (astNodeRef, error) {
	if len(ps) != 0 {
		var name string
		switch k {
		case ASTPrTheoryLemma:
			name = "th-lemma"
		case ASTPrQuantInst:
			if ASTSourceDebug && parents != 0 {
				return astNodeRef{}, ErrASTDomain
			}
			name = "quant-inst"
		case ASTPrHyperResolve:
			name = "hyper-res"
		default:
			return astNodeRef{}, ErrASTDomain
		}
		return m.newProofDecl(name, k, ps, parents, false)
	}
	if k == ASTPrUndef {
		return m.basic.declarations[k], nil
	}
	name, n, array := astProofDescription(k, parents)
	if name == "" {
		return astNodeRef{}, ErrASTDomain
	}
	if array {
		if n == math.MaxUint32 {
			return astNodeRef{}, arena.ErrBounds
		}
		cache := &m.basic.proofCaches[k]
		if e := m.resizeDeclCache(cache, n+1); e != nil {
			return astNodeRef{}, e
		}
		if (*cache)[n].slot == 0 {
			r, e := m.newProofDecl(name, k, nil, n, true)
			if e != nil {
				return astNodeRef{}, e
			}
			(*cache)[n] = r
		}
		return (*cache)[n], nil
	}
	if m.basic.declarations[k].slot == 0 {
		r, e := m.newProofDecl(name, k, nil, n, true)
		if e != nil {
			return astNodeRef{}, e
		}
		m.basic.declarations[k] = r
	}
	return m.basic.declarations[k], nil
}
