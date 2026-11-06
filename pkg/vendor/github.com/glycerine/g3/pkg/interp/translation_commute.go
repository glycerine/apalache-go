package interp

// Source checkpoint: src/interp/iz3translate.cpp commutativity-axiom
// elimination helpers.

import "github.com/glycerine/g3/pkg/ast"

func (t *FullTranslation) makeCommuted(proof, equivalence ast.ASTHandle) (ast.ASTHandle, error) {
	return t.base.manager.MakeDeclaration(t.commute, proof, equivalence)
}

func (t *FullTranslation) isCommuted(value ast.ASTHandle) (bool, error) {
	declaration, err := t.base.manager.Symbol(value)
	return declaration == t.commute, err
}

func (t *FullTranslation) unpackCommuted(sourceProof, translated ast.ASTHandle) (proof, equivalence ast.ASTHandle, result error) {
	commuted, err := t.isCommuted(translated)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if commuted {
		proof, err = t.base.manager.Arg(translated, 0)
		if err == nil {
			equivalence, err = t.base.manager.Arg(translated, 1)
		}
		return proof, equivalence, err
	}
	proof = translated
	equivalence, err = t.base.manager.Conclusion(sourceProof)
	return proof, equivalence, err
}

func (t *FullTranslation) commuteEquality(equality ast.ASTHandle) (ast.ASTHandle, error) {
	right, err := t.base.manager.Arg(equality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(equality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.Make(OperatorEqual, right, left)
}

func (t *FullTranslation) commuteEqualityIff(conclusion ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := t.base.manager.Operator(conclusion)
	if err != nil || op != OperatorIff {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, UnsupportedProofError{Proof: conclusion}
	}
	left, err := t.base.manager.Arg(conclusion, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftOp, e := t.base.manager.Operator(left); e != nil || leftOp != OperatorEqual {
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return ast.ASTHandle{}, UnsupportedProofError{Proof: conclusion}
	}
	right, err := t.base.manager.Arg(conclusion, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = t.commuteEquality(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = t.commuteEquality(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.Make(OperatorIff, left, right)
}

func (t *FullTranslation) twist(proof ast.ASTHandle) (ast.ASTHandle, error) {
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := t.base.manager.Conclusion(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err = t.commuteEqualityIff(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := t.base.manager.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, count+1)
	for i := uint32(0); i < count; i++ {
		premise, e := t.base.manager.Premise(proof, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if rule != ast.ASTPrMonotonicity {
			premise, e = t.twist(premise)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		arguments = append(arguments, premise)
	}
	switch rule {
	case ast.ASTPrMonotonicity, ast.ASTPrSymmetry, ast.ASTPrTransitivity, ast.ASTPrCommutativity:
		arguments = append(arguments, conclusion)
		return t.base.manager.Clone(proof, arguments)
	default:
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
}

func (t *FullTranslation) makeCommutedModusPonens(source ast.ASTHandle, premises []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	proof, equivalence, err := t.unpackCommuted(ast.ASTHandle{}, premises[1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstSource, err := t.base.manager.Premise(source, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p, err := t.base.manager.Conclusion(firstSource)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	q, err := t.base.manager.Conclusion(source)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pCommuted, err := t.base.manager.Arg(equivalence, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	qCommuted, err := t.base.manager.Arg(equivalence, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pProof := premises[0]
	if p != pCommuted {
		pProof, err = direct.MakeSymmetry(pCommuted, p, pProof)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	result, err := direct.MakeModusPonens(equivalence, pProof, proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if q != qCommuted {
		result, err = direct.MakeSymmetry(q, qCommuted, result)
	}
	return result, err
}

func (t *FullTranslation) makeCommutedMonotonicity(premise ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	proof, equivalence, err := t.unpackCommuted(ast.ASTHandle{}, premise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(equivalence, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(equivalence, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = t.base.manager.MkNot(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = t.base.manager.MkNot(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := t.base.manager.Make(OperatorIff, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := direct.MakeCongruence(equivalence, conclusion, proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.makeCommuted(result, conclusion)
}

func (t *FullTranslation) makeCommutedSymmetry(premise ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	proof, equivalence, err := t.unpackCommuted(ast.ASTHandle{}, premise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(equivalence, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(equivalence, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := t.base.manager.Make(OperatorIff, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := direct.MakeSymmetry(conclusion, equivalence, proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.makeCommuted(result, conclusion)
}

func (t *FullTranslation) makeCommutedTransitivity(source ast.ASTHandle, premises []ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, error) {
	source0, err := t.base.manager.Premise(source, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	source1, err := t.base.manager.Premise(source, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proof0, equivalence0, err := t.unpackCommuted(source0, premises[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proof1, equivalence1, err := t.unpackCommuted(source1, premises[1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	middle0, err := t.base.manager.Arg(equivalence0, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	middle1, err := t.base.manager.Arg(equivalence1, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if middle0 != middle1 {
		twisted, e := t.twist(source1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		translated, e := t.translateMain(twisted, false, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof1, equivalence1, err = t.unpackCommuted(twisted, translated)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	left, err := t.base.manager.Arg(equivalence0, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	middle, err := t.base.manager.Arg(equivalence0, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(equivalence1, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := t.base.manager.Make(OperatorIff, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := direct.MakeTransitivity(left, middle, right, proof0, proof1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.makeCommuted(result, conclusion)
}
