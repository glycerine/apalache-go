package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp local arithmetic rule cases.

import "github.com/glycerine/g3/pkg/ast"

func (p *DirectProofInterpolator) MakeFarkas(conclusion ast.ASTHandle, premises, premiseConclusions, coefficients []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(premises) != len(premiseConclusions) || len(coefficients) != len(premiseConclusions) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	var aCoefficients, aInequalities []ast.ASTHandle
	var mixedLiterals []ast.ASTHandle
	for i, literal := range premiseConclusions {
		side, err := p.TermSide(literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if side == TermA {
			aCoefficients = append(aCoefficients, coefficients[i])
			aInequalities = append(aInequalities, literal)
		} else if side == TermMixed {
			mixedLiterals = append(mixedLiterals, literal)
		}
	}
	result, err := p.base.manager.SumInequalities(aCoefficients, aInequalities, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i, literal := range premiseConclusions {
		side, e := p.TermSide(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if side != TermMixed {
			continue
		}
		placeholder, e := p.placeholder(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e = p.makeSymbol(p.sum, result, coefficients[i], placeholder)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	if len(mixedLiterals) != 0 {
		result, err = p.makeContraNode(result, mixedLiterals, -1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	resolutionConclusion := make([]ast.ASTHandle, len(premiseConclusions))
	for i := range premiseConclusions {
		resolutionConclusion[len(premiseConclusions)-i-1] = premiseConclusions[i]
	}
	for i, literal := range premiseConclusions {
		result, err = p.MakeResolution(literal, resolutionConclusion, result, premises[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		resolutionConclusion = resolutionConclusion[:len(resolutionConclusion)-1]
	}
	return p.keep(result, nil)
}

func (p *DirectProofInterpolator) MakeLeqToEq(x, y, xLeqY, yLeqX ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := p.base.manager.Make(OperatorEqual, x, y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermA {
		return p.keep(p.base.manager.Make(OperatorFalse))
	}
	if side == TermB {
		return p.keep(p.base.manager.Make(OperatorTrue))
	}
	negatedConclusion, err := p.base.manager.MkNot(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literals := []ast.ASTHandle{negatedConclusion, xLeqY, yLeqX}
	arguments := make([]ast.ASTHandle, len(literals))
	for i, literal := range literals {
		arguments[i], err = p.placeholder(literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	proof, err := p.makeSymbol(p.leqToEq, arguments...)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.makeContraNode(proof, literals, 1)
	return p.keep(result, err)
}

func (p *DirectProofInterpolator) MakeEqToLeq(x, y, xLeqY ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(xLeqY)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermA {
		return p.keep(p.base.manager.Make(OperatorFalse))
	}
	if side == TermB {
		return p.keep(p.base.manager.Make(OperatorTrue))
	}
	equality, err := p.base.manager.Make(OperatorEqual, x, y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negatedInequality, err := p.base.manager.MkNot(xLeqY)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literals := []ast.ASTHandle{equality, negatedInequality}
	first, err := p.placeholder(literals[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := p.placeholder(literals[1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proof, err := p.makeSymbol(p.eqToLeq, first, second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.makeContraNode(proof, literals, 2)
	return p.keep(result, err)
}

func (p *DirectProofInterpolator) MakeCutRule(tLeqC, divisor, conclusion, premise ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var interpolation ast.ASTHandle
	if side == TermA {
		interpolation, err = p.base.manager.Make(OperatorFalse)
	} else if side == TermB {
		interpolation, err = p.base.manager.Make(OperatorTrue)
	} else {
		negatedConclusion, e := p.base.manager.MkNot(conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		first, e := p.placeholder(tLeqC)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		second, e := p.placeholder(negatedConclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		interpolation, e = p.makeSymbol(p.sum, first, divisor, second)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		interpolation, err = p.makeContraNode(interpolation, []ast.ASTHandle{tLeqC, negatedConclusion}, -1)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.MakeResolution(tLeqC, []ast.ASTHandle{conclusion}, interpolation, premise)
}
