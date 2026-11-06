package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp mixed contradiction
// recognition and resolvent collection.

import "github.com/glycerine/g3/pkg/ast"

func (p *DirectProofInterpolator) isNegativeEquality(term ast.ASTHandle) (bool, error) {
	op, err := p.base.manager.Operator(term)
	if err != nil || op != OperatorNot {
		return false, err
	}
	body, err := p.base.manager.Arg(term, 0)
	if err != nil {
		return false, err
	}
	op, err = p.base.manager.Operator(body)
	return op == OperatorEqual || op == OperatorIff, err
}

func (p *DirectProofInterpolator) countNegativeEqualities(resolvent []ast.ASTHandle) (int, error) {
	count := 0
	for _, entry := range resolvent {
		literal, err := p.base.manager.Arg(entry, 1)
		if err != nil {
			return 0, err
		}
		negative, err := p.isNegativeEquality(literal)
		if err != nil {
			return 0, err
		}
		if negative {
			count++
		}
	}
	return count, nil
}

func (p *DirectProofInterpolator) contraPivot(target, interpolation ast.ASTHandle) (ast.ASTHandle, bool, error) {
	op, err := p.base.manager.Operator(interpolation)
	if err != nil || op != OperatorAnd {
		return ast.ASTHandle{}, false, err
	}
	count, err := p.base.manager.NumArgs(interpolation)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	for i := uint32(1); i < count; i++ {
		entry, e := p.base.manager.Arg(interpolation, i)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		entryOp, e := p.base.manager.Operator(entry)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if entryOp != OperatorUninterpreted {
			break
		}
		declaration, e := p.base.manager.Symbol(entry)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if declaration != p.contra {
			break
		}
		literal, e := p.base.manager.Arg(entry, 1)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if literal == target {
			return entry, true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (p *DirectProofInterpolator) collectContraResolvents(from uint32, pivot1, pivot, conjunction ast.ASTHandle, result *[]ast.ASTHandle) error {
	count, err := p.base.manager.NumArgs(conjunction)
	if err != nil {
		return err
	}
	negatedLiteral, err := p.base.manager.Arg(pivot1, 1)
	if err != nil {
		return err
	}
	negatedLiteral, err = p.base.manager.MkNot(negatedLiteral)
	if err != nil {
		return err
	}
	placeholder, err := p.placeholder(negatedLiteral)
	if err != nil {
		return err
	}
	proof, err := p.base.manager.Arg(pivot1, 0)
	if err != nil {
		return err
	}
	for i := from; i < count; i++ {
		entry, e := p.base.manager.Arg(conjunction, i)
		if e != nil {
			return e
		}
		if entry == pivot {
			continue
		}
		entryProof, e := p.base.manager.Arg(entry, 0)
		if e != nil {
			return e
		}
		if proof != p.noProof {
			entryProof, e = p.SubstituteProofTerm(placeholder, proof, entryProof)
			if e != nil {
				return e
			}
		} else {
			entryProof = p.noProof
		}
		literal, e := p.base.manager.Arg(entry, 1)
		if e != nil {
			return e
		}
		resolved, e := p.makeSymbol(p.contra, entryProof, literal)
		if e != nil {
			return e
		}
		*result = append(*result, resolved)
	}
	return nil
}

func (p *DirectProofInterpolator) getLiteralAtom(literal ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorNot {
		return p.base.manager.Arg(literal, 0)
	}
	return literal, nil
}

func (p *DirectProofInterpolator) resolveArithmetic(pivot ast.ASTHandle, conclusion []ast.ASTHandle, premise1, premise2 ast.ASTHandle) (ast.ASTHandle, error) {
	atom, err := p.getLiteralAtom(pivot)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativePivot, err := p.base.manager.MkNot(atom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(pivot)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op != OperatorNot {
		premise1, premise2 = premise2, premise1
	}
	if op == OperatorEqual {
		left, e := p.base.manager.Arg(pivot, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := p.base.manager.Arg(pivot, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		leftOp, e := p.base.manager.Operator(left)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rightOp, e := p.base.manager.Operator(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if leftOp == OperatorSelect && rightOp == OperatorSelect {
			negativePivot, err = p.base.manager.MkNot(negativePivot)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			premise1, premise2 = premise2, premise1
		}
	}
	return p.resolveArithmeticFirst(make(map[ast.ASTHandle]ast.ASTHandle), negativePivot, premise1, premise2)
}

func (p *DirectProofInterpolator) recurseResolutionTerm(current ast.ASTHandle, recurse func(ast.ASTHandle) (ast.ASTHandle, error)) (ast.ASTHandle, bool, error) {
	op, err := p.base.manager.Operator(current)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if op == OperatorAnd || op == OperatorOr || op == OperatorImplies {
		args, e := p.base.manager.Args(current)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		for i := range args {
			args[i], e = recurse(args[i])
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
		}
		result, e := p.base.manager.Clone(current, args)
		return result, true, e
	}
	if op == OperatorUninterpreted {
		declaration, e := p.base.manager.Symbol(current)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if declaration == p.standForall || declaration == p.standExists {
			binding, e := p.base.manager.Arg(current, 0)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			body, e := p.base.manager.Arg(current, 1)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			body, e = recurse(body)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			result, e := p.makeSymbol(declaration, binding, body)
			return result, true, e
		}
	}
	return current, false, nil
}

func (p *DirectProofInterpolator) resolveArithmeticFirst(memo map[ast.ASTHandle]ast.ASTHandle, negativePivot, interpolation1, interpolation2 ast.ASTHandle) (ast.ASTHandle, error) {
	if result, found := memo[interpolation1]; found {
		return result, nil
	}
	pivot1, found, err := p.contraPivot(negativePivot, interpolation1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	if found {
		result, err = p.resolveArithmeticSecond(make(map[ast.ASTHandle]ast.ASTHandle), pivot1, interpolation1, interpolation2)
	} else {
		result, _, err = p.recurseResolutionTerm(interpolation1, func(child ast.ASTHandle) (ast.ASTHandle, error) {
			return p.resolveArithmeticFirst(memo, negativePivot, child, interpolation2)
		})
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	memo[interpolation1] = result
	return result, nil
}

func (p *DirectProofInterpolator) resolveArithmeticSecond(memo map[ast.ASTHandle]ast.ASTHandle, pivot1, conjunction1, interpolation2 ast.ASTHandle) (ast.ASTHandle, error) {
	if result, found := memo[interpolation2]; found {
		return result, nil
	}
	literal, err := p.base.manager.Arg(pivot1, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	target, err := p.base.manager.MkNot(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pivot2, found, err := p.contraPivot(target, interpolation2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	if found {
		result, err = p.resolveContradictions(pivot1, conjunction1, pivot2, interpolation2)
	} else {
		result, _, err = p.recurseResolutionTerm(interpolation2, func(child ast.ASTHandle) (ast.ASTHandle, error) {
			return p.resolveArithmeticSecond(memo, pivot1, conjunction1, child)
		})
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	memo[interpolation2] = result
	return result, nil
}

func (p *DirectProofInterpolator) resolveContradictions(pivot1, conjunction1, pivot2, conjunction2 ast.ASTHandle) (ast.ASTHandle, error) {
	proof1, err := p.base.manager.Arg(pivot1, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proof2, err := p.base.manager.Arg(pivot2, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if proof1 == p.noProof && proof2 == p.noProof {
		return p.resolveWithLocalization(pivot1, conjunction1, pivot2, conjunction2)
	}
	if proof1 == p.noProof {
		pivot1, conjunction1, pivot2, conjunction2 = pivot2, conjunction2, pivot1, conjunction1
	}
	var resolvent []ast.ASTHandle
	if err = p.collectContraResolvents(0, pivot1, pivot2, conjunction2, &resolvent); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.collectContraResolvents(1, pivot2, pivot1, conjunction1, &resolvent); err != nil {
		return ast.ASTHandle{}, err
	}
	negativeEqualities, err := p.countNegativeEqualities(resolvent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if negativeEqualities > 1 || len(resolvent) == 0 {
		return ast.ASTHandle{}, ProofError{}
	}
	if len(resolvent) == 1 {
		proof, e := p.base.manager.Arg(resolvent[0], 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.simplifyProofTerm(proof)
	}
	return p.base.manager.Make(OperatorAnd, resolvent...)
}

func (p *DirectProofInterpolator) resolveWithLocalization(pivot1, conjunction1, pivot2, conjunction2 ast.ASTHandle) (ast.ASTHandle, error) {
	literal, err := p.base.manager.Arg(pivot1, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorNot {
		return p.resolveWithLocalization(pivot2, conjunction2, pivot1, conjunction1)
	}
	localized, equalityProof, err := p.localizeTerm(literal, p.scope)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	localizedHypothesis, err := p.MakeHypothesis(localized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equality, err := p.base.manager.Make(OperatorIff, localized, literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveProof, err := p.MakeModusPonens(equality, localizedHypothesis, equalityProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveResult, err := p.MakeResolution(literal, nil, conjunction1, positiveProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeLiteral, err := p.base.manager.MkNot(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeLocalized, err := p.base.manager.MkNot(localized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeEquality, err := p.base.manager.Make(OperatorIff, negativeLocalized, negativeLiteral)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeEqualityProof, err := p.MakeCongruence(equality, negativeEquality, equalityProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeHypothesis, err := p.MakeHypothesis(negativeLocalized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeProof, err := p.MakeModusPonens(negativeEquality, negativeHypothesis, negativeEqualityProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeResult, err := p.MakeResolution(negativeLiteral, nil, conjunction2, negativeProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.MakeResolution(localized, nil, positiveResult, negativeResult)
}
