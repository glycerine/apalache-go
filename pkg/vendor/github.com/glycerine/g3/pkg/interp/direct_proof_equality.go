package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp local modus-ponens,
// congruence, and transitivity rules.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *DirectProofInterpolator) makeEquivalence(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	sort, err := p.base.manager.Type(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := p.base.manager.IsBoolType(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if boolean {
		return p.base.manager.Make(OperatorIff, left, right)
	}
	return p.base.manager.Make(OperatorEqual, left, right)
}

func (p *DirectProofInterpolator) MakeModusPonens(equivalence, premise1, premise2 ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(equivalence)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := p.base.manager.Arg(equivalence, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.base.manager.Arg(equivalence, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermMixed {
		negatedRight, e := p.base.manager.MkNot(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		mask := 1 << 2
		leftOp, e := p.base.manager.Operator(left)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if leftOp == OperatorNot {
			body, bodyErr := p.base.manager.Arg(left, 0)
			if bodyErr != nil {
				return ast.ASTHandle{}, bodyErr
			}
			negativeEquivalence, bodyErr := p.isEquivalence(body)
			if bodyErr != nil {
				return ast.ASTHandle{}, bodyErr
			}
			if negativeEquivalence {
				mask |= 1
			}
		}
		interpolation, e := p.trivialInterpolation(p.modusPonens, []ast.ASTHandle{left, equivalence, negatedRight}, mask)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negatedEquivalence, e := p.base.manager.MkNot(equivalence)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		interpolation, e = p.MakeResolution(left, []ast.ASTHandle{right, negatedEquivalence}, interpolation, premise1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.MakeResolution(equivalence, []ast.ASTHandle{right}, interpolation, premise2)
	}
	leftSide, err := p.TermSide(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSide, err := p.TermSide(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var interpolation ast.ASTHandle
	if leftSide == TermA {
		if rightSide == TermA {
			rightOp, e := p.base.manager.Operator(right)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if rightOp == OperatorOr {
				args, e := p.base.manager.Args(right)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				interpolation, err = p.MakeAssumption(p.scope.Hi, args)
			} else {
				interpolation, err = p.base.manager.Make(OperatorFalse)
			}
		} else if side == TermA {
			interpolation = right
		} else {
			return ast.ASTHandle{}, ProofError{}
		}
	} else if rightSide == TermA {
		comparison, e := p.base.manager.Make(OperatorEqual, left, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		comparisonSide, e := p.TermSide(comparison)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if comparisonSide != TermA {
			return ast.ASTHandle{}, ProofError{}
		}
		interpolation, err = p.base.manager.MkNot(left)
	} else {
		interpolation, err = p.base.manager.Make(OperatorTrue)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negatedEquivalence, err := p.base.manager.MkNot(equivalence)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err = p.MakeResolution(left, []ast.ASTHandle{right, negatedEquivalence}, interpolation, premise1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.MakeResolution(equivalence, []ast.ASTHandle{right}, interpolation, premise2)
}

func (p *DirectProofInterpolator) MakeCongruence(premiseConclusion, conclusion, premise ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	premiseSide, err := p.TermSide(premiseConclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusionSide, err := p.TermSide(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if conclusionSide == TermMixed {
		interpolation, e := p.makeMixedCongruence(premiseConclusion, conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.MakeResolution(premiseConclusion, []ast.ASTHandle{conclusion}, interpolation, premise)
	}
	if premiseSide == TermMixed || conclusionSide == TermMixed {
		return ast.ASTHandle{}, ProofError{}
	}
	var interpolation ast.ASTHandle
	if premiseSide == TermA {
		if conclusionSide == TermA {
			interpolation, err = p.base.manager.Make(OperatorFalse)
		} else {
			interpolation = premiseConclusion
		}
	} else if conclusionSide == TermA {
		interpolation, err = p.base.manager.MkNot(premiseConclusion)
	} else {
		interpolation, err = p.base.manager.Make(OperatorTrue)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.MakeResolution(premiseConclusion, []ast.ASTHandle{conclusion}, interpolation, premise)
}

func (p *DirectProofInterpolator) makeMixedCongruence(premiseConclusion, conclusion ast.ASTHandle) (ast.ASTHandle, error) {
	position, err := p.findCongruencePosition(premiseConclusion, conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	premiseSide, err := p.TermSide(premiseConclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var proof ast.ASTHandle
	literals := make([]ast.ASTHandle, 0, 2)
	if premiseSide == TermMixed {
		proof, err = p.placeholder(premiseConclusion)
		literals = append(literals, premiseConclusion)
	} else {
		proof, err = p.makeLocalRewrite(premiseSide, premiseConclusion)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positionTerm, err := p.base.manager.MakeIntText(strconv.Itoa(position))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negatedConclusion, err := p.base.manager.MkNot(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusionPlaceholder, err := p.placeholder(negatedConclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proof, err = p.makeSymbol(p.congruence, proof, positionTerm, conclusionPlaceholder)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literals = append(literals, negatedConclusion)
	return p.makeContraNode(proof, literals, -1)
}

func (p *DirectProofInterpolator) findCongruencePosition(premiseConclusion, conclusion ast.ASTHandle) (int, error) {
	x, err := p.base.manager.Arg(premiseConclusion, 0)
	if err != nil {
		return 0, err
	}
	y, err := p.base.manager.Arg(premiseConclusion, 1)
	if err != nil {
		return 0, err
	}
	left, err := p.base.manager.Arg(conclusion, 0)
	if err != nil {
		return 0, err
	}
	right, err := p.base.manager.Arg(conclusion, 1)
	if err != nil {
		return 0, err
	}
	count, err := p.base.manager.NumArgs(left)
	if err != nil {
		return 0, err
	}
	for i := uint32(0); i < count; i++ {
		leftArg, e := p.base.manager.Arg(left, i)
		if e != nil {
			return 0, e
		}
		rightArg, e := p.base.manager.Arg(right, i)
		if e != nil {
			return 0, e
		}
		if leftArg == x && rightArg == y {
			return int(i), nil
		}
	}
	return 0, ProofError{}
}

func (p *DirectProofInterpolator) substituteArgument(position int, term, application ast.ASTHandle) (ast.ASTHandle, error) {
	args, err := p.base.manager.Args(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if position < 0 || position >= len(args) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	args[position] = term
	return p.base.manager.Clone(application, args)
}

func (p *DirectProofInterpolator) MakeCongruences(premiseConclusions []ast.ASTHandle, conclusion ast.ASTHandle, premises []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(premiseConclusions) == 0 || len(premiseConclusions) != len(premises) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if len(premiseConclusions) == 1 {
		return p.MakeCongruence(premiseConclusions[0], conclusion, premises[0])
	}
	thing := conclusion
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := truth
	for i, premiseConclusion := range premiseConclusions {
		position, e := p.findCongruencePosition(premiseConclusion, thing)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rightTerm, e := p.base.manager.Arg(premiseConclusion, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		thingLeft, e := p.base.manager.Arg(thing, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		next, e := p.substituteArgument(position, rightTerm, thingLeft)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		thingOp, e := p.base.manager.Operator(thing)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		goal, e := p.base.manager.Make(thingOp, thingLeft, next)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equality, e := p.MakeCongruence(premiseConclusion, goal, premises[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if i == 0 {
			result = equality
		} else {
			conclusionLeft, e := p.base.manager.Arg(conclusion, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			thingCurrentLeft, e := p.base.manager.Arg(thing, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			conclusionOp, e := p.base.manager.Operator(conclusion)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			trace, e := p.base.manager.Make(conclusionOp, conclusionLeft, thingCurrentLeft)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			traceLeft, e := p.base.manager.Arg(trace, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			updatedTrace, e := p.base.manager.Make(conclusionOp, traceLeft, next)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			equivalence, e := p.base.manager.Make(OperatorIff, trace, updatedTrace)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			proof, e := p.MakeCongruence(goal, equivalence, equality)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = p.MakeModusPonens(equivalence, result, proof)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		thingRight, e := p.base.manager.Arg(thing, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		thing, e = p.base.manager.Make(thingOp, next, thingRight)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return p.keep(result, nil)
}

func (p *DirectProofInterpolator) MakeTransitivity(x, y, z, premise1, premise2 ast.ASTHandle) (ast.ASTHandle, error) {
	left, err := p.makeEquivalence(x, y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.makeEquivalence(y, z)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := p.makeEquivalence(x, z)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equivalence, err := p.base.manager.Make(OperatorIff, left, conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err := p.MakeCongruence(right, equivalence, premise2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.MakeModusPonens(equivalence, premise1, interpolation)
}
