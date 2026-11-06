package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp local resolution dispatch.

import "github.com/glycerine/g3/pkg/ast"

// MakeResolution applies the source Boolean interpolation rule for local A or
// B pivots. Mixed pivots are handled by the arithmetic cut-elimination path,
// which remains a separate porting stage.
func (p *DirectProofInterpolator) MakeResolution(pivot ast.ASTHandle, conclusion []ast.ASTHandle, premise1, premise2 ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(pivot)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch side {
	case TermA:
		return p.keep(p.base.manager.MkOr(premise1, premise2))
	case TermB:
		return p.keep(p.base.manager.MkAnd(premise1, premise2))
	default:
		result, err := p.resolveArithmetic(pivot, conclusion, premise1, premise2)
		return p.keep(result, err)
	}
}

// MakeSymmetry preserves local equality proofs unchanged. Mixed symmetry uses
// the mixed-resolution machinery and is completed with that rule family.
func (p *DirectProofInterpolator) MakeSymmetry(conclusion, premiseConclusion, premise ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermMixed {
		negatedConclusion, e := p.base.manager.MkNot(conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		premisePlaceholder, e := p.placeholder(premiseConclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negatedPlaceholder, e := p.placeholder(negatedConclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		symmetricPremise, e := p.makeSymbol(p.symmetry, premisePlaceholder)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		excludedMiddle, e := p.makeSymbol(p.excludedMid, conclusion, symmetricPremise, negatedPlaceholder)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		falsehood, e := p.base.manager.Make(OperatorFalse)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		first, e := p.makeSymbol(p.contra, excludedMiddle, falsehood)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negatedSymmetry, e := p.makeSymbol(p.symmetry, negatedPlaceholder)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		second, e := p.makeSymbol(p.contra, negatedSymmetry, premiseConclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		third, e := p.makeSymbol(p.contra, symmetricPremise, negatedConclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		interpolation, e := p.base.manager.Make(OperatorAnd, first, second, third)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.MakeResolution(premiseConclusion, []ast.ASTHandle{conclusion}, interpolation, premise)
	}
	return p.keep(premise, nil)
}
