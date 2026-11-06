package ast

// Source: src/ast/bv_decl_plugin.cpp:132-141,437-468.

import (
	"math"
)

func (m *ASTManager) bitVectorMKBV(domain []ASTHandle) (astNodeRef, error) {
	p := m.bitVectorPlugin
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	for _, sort := range domain {
		r, e := m.requireKind(sort, ASTSort)
		if e != nil {
			return astNodeRef{}, e
		}
		if r != m.boolSort {
			return astNodeRef{}, bvDeclarationError("invalid mkbv operator")
		}
	}
	if uint64(len(domain)) >= math.MaxUint32 {
		return astNodeRef{}, ErrASTDomain
	}
	if p.fixedPending.slot != 0 {
		return astNodeRef{}, ErrASTPending
	}
	width := uint32(len(domain))
	v := &p.fixed[56]
	if uint64(len(*v)) < uint64(width+1) {
		*v = append(*v, make([]astNodeRef, int(width+1)-len(*v))...)
	}
	r := (*v)[width]
	if r.slot != 0 {
		return r, nil
	}
	result, e := p.getSort(width)
	if e != nil {
		return astNodeRef{}, e
	}
	r, e = m.pluginDecl(p.names[9], p.family, 56, nil, domain, result, ASTFuncFlags{}, false)
	if e != nil {
		return astNodeRef{}, e
	}
	return r, p.publishFixed(56, width, r)
}

func (m *ASTManager) bitVectorBit2Bool(width uint32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	p := m.bitVectorPlugin
	if len(ps) != 1 {
		return astNodeRef{}, bvDeclarationError("invalid bit2bool declaration")
	}
	parameter, e := m.parameter(ps[0])
	if e != nil {
		return astNodeRef{}, e
	}
	if parameter.Kind != ASTParameterInt || len(domain) != 1 || parameter.integer >= int32(width) {
		return astNodeRef{}, bvDeclarationError("invalid bit2bool declaration")
	}
	if width == math.MaxUint32 {
		return astNodeRef{}, ErrASTDomain
	}
	if p.fixedPending.slot != 0 {
		return astNodeRef{}, ErrASTPending
	}
	if uint64(len(p.bit2bool)) < uint64(width)+1 {
		p.bit2bool = append(p.bit2bool, make([][]astNodeRef, int(width)+1-len(p.bit2bool))...)
	}
	v := &p.bit2bool[width]
	if uint64(len(*v)) < uint64(width) {
		*v = append(*v, make([]astNodeRef, int(width)-len(*v))...)
	}
	index := uint32(parameter.integer)
	// Negative indices pass the original signed comparison, then convert to
	// unsigned and read outside the source vector. Reject that undefined access
	// after preserving the reached outer/inner resize effects.
	if index >= width {
		return astNodeRef{}, ErrASTDomain
	}
	r := (*v)[index]
	if r.slot != 0 {
		return r, nil
	}
	r, e = m.pluginDecl(p.names[8], p.family, 55, ps, domain, m.boolSort, ASTFuncFlags{}, false)
	if e != nil {
		return astNodeRef{}, e
	}
	(*v)[index] = r
	p.fixedPending, p.fixedPendingKind, p.fixedPendingWidth, p.fixedPendingIndex = r, 55, width, index
	if e = m.incNode(r); e != nil {
		return r, e
	}
	p.fixedPending = astNodeRef{}
	return r, nil
}

func (p *astBitVectorPluginOwner) releaseBooleanBits() {
	p.bit2bool = nil
	p.fixed[56] = nil
}
func (p *astBitVectorPluginOwner) finalizeBooleanBits() error {
	for width := range p.bit2bool {
		if e := p.finalizeBitVectorCache(&p.bit2bool[width]); e != nil {
			return e
		}
	}
	return p.finalizeBitVectorCache(&p.fixed[56])
}
