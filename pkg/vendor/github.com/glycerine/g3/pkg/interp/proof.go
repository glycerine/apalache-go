package interp

// Source checkpoint: the retained iz3proof.cpp/iz3proof.h path used by
// iz3translation_full::translate and iz3interp::proof_to_interpolant.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type ProofNode int

const NullProofNode ProofNode = -1

type ProofError struct{}

func (ProofError) Error() string { return "proof_error" }

type interpolationProofNode struct {
	conclusion   []ast.ASTHandle
	interpolants []ast.ASTHandle
}

// InterpolationProof is the concrete proof form produced by the retained full
// translator: a lemma containing the already computed per-frame interpolants.
// It borrows Base and owns roots for all stored formulas.
type InterpolationProof struct {
	self   *InterpolationProof
	base   *Base
	nodes  []interpolationProofNode
	owned  []ast.ASTRef
	closed bool
}

func NewInterpolationProof(base *Base) (*InterpolationProof, error) {
	if base == nil || base.open() != nil {
		return nil, ast.ErrASTHandle
	}
	p := &InterpolationProof{base: base}
	p.self = p
	return p, nil
}

func (p *InterpolationProof) open() error {
	if p == nil || p.self != p || p.closed || p.base == nil {
		return ast.ErrASTHandle
	}
	return p.base.open()
}

func (p *InterpolationProof) hold(handle ast.ASTHandle) error {
	ref, err := p.base.manager.ast.NewRef(handle)
	if err != nil {
		return err
	}
	p.owned = append(p.owned, ref)
	return nil
}

func (p *InterpolationProof) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for i := len(p.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, p.base.manager.ast.DelRef(&p.owned[i]))
	}
	p.owned = nil
	p.nodes = nil
	p.closed = true
	return result
}

// MakeLemma stores the empty-clause lemma produced by the retained full
// translator. Nonempty conclusions are accepted because the source operation
// is also used by its debugger helpers, although the public path supplies nil.
func (p *InterpolationProof) MakeLemma(conclusion, interpolants []ast.ASTHandle) (node ProofNode, resultErr error) {
	if err := p.open(); err != nil {
		return NullProofNode, err
	}
	ownedStart := len(p.owned)
	defer func() {
		if resultErr != nil {
			for i := len(p.owned) - 1; i >= ownedStart; i-- {
				resultErr = errors.Join(resultErr, p.base.manager.ast.DelRef(&p.owned[i]))
			}
			p.owned = p.owned[:ownedStart]
		}
	}()
	for _, handle := range conclusion {
		if err := p.hold(handle); err != nil {
			return NullProofNode, err
		}
	}
	for _, handle := range interpolants {
		if err := p.hold(handle); err != nil {
			return NullProofNode, err
		}
	}
	n := interpolationProofNode{
		conclusion:   append([]ast.ASTHandle(nil), conclusion...),
		interpolants: append([]ast.ASTHandle(nil), interpolants...),
	}
	p.nodes = append(p.nodes, n)
	return ProofNode(len(p.nodes) - 1), nil
}

func (p *InterpolationProof) Conclusion(node ProofNode) ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if node < 0 || int(node) >= len(p.nodes) {
		return nil, ast.ErrASTDomain
	}
	return append([]ast.ASTHandle(nil), p.nodes[node].conclusion...), nil
}

// Interpolate selects the lemma result for the maximum frame in scope and
// applies the source Base simplifier. The returned reference is caller-owned.
// Weak mode does not alter a precomputed lemma result.
func (p *InterpolationProof) Interpolate(node ProofNode, scope ScopeRange, weak bool) (ast.ASTRef, error) {
	if err := p.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if node < 0 || int(node) >= len(p.nodes) || scope.Hi < 0 || scope.Hi >= len(p.nodes[node].interpolants) {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	result, err := p.base.Simplify(p.nodes[node].interpolants[scope.Hi])
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.base.manager.ast.NewRef(result)
}
