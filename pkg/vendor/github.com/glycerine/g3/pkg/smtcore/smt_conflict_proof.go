package smtcore

// Source: Boolean and theory-literal portions of
// src/smt/smt_conflict_resolution.cpp mk_conflict_proof() and
// src/smt/smt_justification.cpp mk_proof methods.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtConflictProofBuilder struct {
	state    *SMTInternalizerState
	marked   map[int32]struct{}
	literal  map[SMTLiteral]ast.ASTHandle
	objects  map[SMTJustification]ast.ASTHandle
	equality map[smtEnodePairKey]ast.ASTHandle
}

func (b *smtConflictProofBuilder) normalizeEqualityProof(first, second *SMTEnode, proof ast.ASTHandle) (ast.ASTHandle, error) {
	if first == nil || second == nil || proof.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	fact, err := b.state.manager.GetProofFact(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := b.state.manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if fact == first.owner && second.owner == truth {
		return b.state.manager.MkProofIffTrue(proof)
	}
	falsity, err := b.state.manager.MkFalse()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if second.owner == falsity {
		negated, child, e := b.state.manager.IsNot(fact)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if negated && child == first.owner {
			return b.state.manager.MkProofIffFalse(proof)
		}
	}
	count, err := b.state.manager.GetNumArgs(fact)
	if err != nil || count != 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	left, err := b.state.manager.GetArg(fact, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := b.state.manager.GetArg(fact, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if left == first.owner && right == second.owner {
		return proof, nil
	}
	if left == second.owner && right == first.owner {
		return b.state.manager.MkProofSymmetry(proof)
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}

func (b *smtConflictProofBuilder) equalityJustificationProof(first, second *SMTEnode, justification SMTEqualityJustification) (ast.ASTHandle, error) {
	switch justification.Kind() {
	case SMTEqualityJustificationEquation:
		literal, ok := justification.Literal()
		if !ok {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		proof, err := b.literalProof(literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return b.normalizeEqualityProof(first, second, proof)
	case SMTEqualityJustificationObject:
		object, ok := justification.Justification()
		if !ok || object == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		proof, err := b.objectProof(object)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return b.normalizeEqualityProof(first, second, proof)
	case SMTEqualityJustificationCongruence:
		commuted, ok := justification.UsedCommutativity()
		if !ok || len(first.arguments) != len(second.arguments) || first.declaration != second.declaration {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if commuted {
			if len(first.arguments) != 2 {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			parents := make([]ast.ASTHandle, 0, 2)
			for _, pair := range [][2]*SMTEnode{{first.arguments[0], second.arguments[1]}, {first.arguments[1], second.arguments[0]}} {
				if pair[0] != pair[1] {
					proof, err := b.equalityProof(pair[0], pair[1])
					if err != nil {
						return ast.ASTHandle{}, err
					}
					parents = append(parents, proof)
				}
			}
			secondPrime, err := b.state.manager.MkApp(second.declaration, []ast.ASTHandle{second.arguments[1].owner, second.arguments[0].owner})
			if err != nil {
				return ast.ASTHandle{}, err
			}
			var congruence ast.ASTHandle
			if len(parents) != 0 {
				congruence, err = b.state.manager.MkProofCongruence(first.owner, secondPrime, parents)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			commutativity, err := b.state.manager.MkProofCommutativity(secondPrime)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return b.state.manager.MkProofTransitivity(congruence, commutativity)
		}
		parents := make([]ast.ASTHandle, 0, len(first.arguments))
		for i := range first.arguments {
			if first.arguments[i] != second.arguments[i] {
				proof, err := b.equalityProof(first.arguments[i], second.arguments[i])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				parents = append(parents, proof)
			}
		}
		return b.state.manager.MkProofCongruence(first.owner, second.owner, parents)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}

func (b *smtConflictProofBuilder) equalityBranchProofs(node, ancestor *SMTEnode) ([]ast.ASTHandle, error) {
	if node == nil || ancestor == nil || !node.TransReaches(ancestor) {
		return nil, ast.ErrASTDomain
	}
	proofs := make([]ast.ASTHandle, 0)
	for node != ancestor {
		next := node.trans.Target
		if next == nil {
			return nil, ast.ErrASTDomain
		}
		proof, err := b.equalityJustificationProof(node, next, node.trans.Justification)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
		node = next
	}
	return proofs, nil
}

func (b *smtConflictProofBuilder) equalityProof(first, second *SMTEnode) (ast.ASTHandle, error) {
	if first == nil || second == nil || first == second || first.root == nil || first.root != second.root {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	key := smtEnodePairKey{first, second}
	if proof, found := b.equality[key]; found {
		return proof, nil
	}
	common, err := smtCommonTransAncestor(first, second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proofs, err := b.equalityBranchProofs(first, common)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	other, err := b.equalityBranchProofs(second, common)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := len(other) - 1; i >= 0; i-- {
		proof, e := b.state.manager.MkProofSymmetry(other[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proofs = append(proofs, proof)
	}
	if len(proofs) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	proof := proofs[0]
	if len(proofs) > 1 {
		proof, err = b.state.manager.MkProofTransitivityStar(proofs, first.owner, second.owner)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	b.equality[key] = proof
	return proof, nil
}

func (b *smtConflictProofBuilder) literalExpression(literal SMTLiteral) (ast.ASTHandle, error) {
	return b.state.literalExpression(literal)
}

func (b *smtConflictProofBuilder) clauseProof(clause *SMTClause) (ast.ASTHandle, error) {
	if clause == nil || clause.justification == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return b.objectProof(clause.justification)
}

func (b *smtConflictProofBuilder) literalProof(literal SMTLiteral) (ast.ASTHandle, error) {
	// Source context initialization gives true_bool_var a true-proof
	// justification. The Go state keeps this reserved variable out of the
	// ordinary Boolean data table.
	if literal == SMTTrueLiteral {
		return b.state.manager.MkProofTrue()
	}
	if proof, found := b.literal[literal]; found {
		return proof, nil
	}
	expression, err := b.literalExpression(literal)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variable := literal.Var()
	justification := b.state.booleanData[variable].Justification()
	if _, hypothesis := b.marked[variable]; hypothesis || justification.Kind() == SMTBooleanJustificationAxiom {
		proof, err := b.state.manager.MkProofHypothesis(expression)
		if err == nil {
			b.literal[literal] = proof
		}
		return proof, err
	}
	proof, err := b.booleanProof(literal, justification)
	if err == nil {
		b.literal[literal] = proof
	}
	return proof, err
}

func (b *smtConflictProofBuilder) booleanProof(consequent SMTLiteral, justification SMTBooleanJustification) (ast.ASTHandle, error) {
	parents := make([]ast.ASTHandle, 0)
	switch justification.Kind() {
	case SMTBooleanJustificationClause:
		clause, ok := justification.Clause()
		if !ok || clause == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		proof, err := b.clauseProof(clause)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		parents = append(parents, proof)
		position := -1
		if consequent != SMTFalseLiteral {
			for i := 0; i < len(clause.literals) && i < 2; i++ {
				if clause.literals[i] == consequent {
					position = i
					break
				}
			}
			if position < 0 {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
		}
		for i, literal := range clause.literals {
			if i == position {
				continue
			}
			antecedent, err := b.literalProof(literal.Negated())
			if err != nil {
				return ast.ASTHandle{}, err
			}
			parents = append(parents, antecedent)
		}
	case SMTBooleanJustificationObject:
		object, ok := justification.Justification()
		if !ok || object == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return b.objectProof(object)
	case SMTBooleanJustificationAxiom:
		fact, err := b.literalExpression(consequent)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return b.state.manager.MkProofHypothesis(fact)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	fact, err := b.literalExpression(consequent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.state.manager.MkProofUnitResolutionFact(parents, fact)
}

func (b *smtConflictProofBuilder) literalProofs(literals []SMTLiteral) ([]ast.ASTHandle, error) {
	proofs := make([]ast.ASTHandle, len(literals))
	for i, literal := range literals {
		proof, err := b.literalProof(literal)
		if err != nil {
			return nil, err
		}
		proofs[i] = proof
	}
	return proofs, nil
}

func (b *smtConflictProofBuilder) theoryFact(literals []SMTLiteral) (ast.ASTHandle, error) {
	expressions := make([]ast.ASTHandle, len(literals))
	for i, literal := range literals {
		expression, err := b.literalExpression(literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		expressions[i] = expression
	}
	if len(expressions) == 1 {
		return expressions[0], nil
	}
	return b.state.manager.MkOrArgs(expressions)
}

func (b *smtConflictProofBuilder) objectProof(object SMTJustification) (ast.ASTHandle, error) {
	if proof, found := b.objects[object]; found {
		return proof, nil
	}
	var proof ast.ASTHandle
	var err error
	switch value := object.(type) {
	case *SMTJustificationProofWrapper:
		proof, err = value.Proof()
	case *smtBVFixedEqualityJustification:
		proof, err = b.bvFixedEqualityProof(value)
	case *smtBVBitEqualityJustification:
		proof, err = b.bvBitEqualityProof(value)
	case *smtDynamicAckJustification:
		proof, err = b.dynamicAckProof(value)
	case *SMTUnitResolutionJustification:
		if value.Antecedent == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		first, e := b.objectProof(value.Antecedent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		parents, e := b.literalProofs(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofUnitResolution(append([]ast.ASTHandle{first}, parents...))
	case *SMTTheoryAxiomJustification:
		fact, e := b.theoryFact(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, nil, value.Parameters())
	case *SMTTheoryPropagationJustification:
		parents, e := b.literalProofs(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		fact, e := b.literalExpression(value.Consequent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, parents, value.Parameters())
	case *smtPBJustification:
		proof, err = b.objectProof(value.SMTTheoryPropagationJustification)
	case *SMTTheoryConflictJustification:
		parents, e := b.literalProofs(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		fact, e := b.state.manager.MkFalse()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, parents, value.Parameters())
	case *SMTEqualityPropagationJustification:
		proof, err = b.equalityProof(value.First, value.Second)
	case *SMTEqualityRootJustification:
		if value.Node == nil || value.Node.root == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		first, e := b.equalityProof(value.Node, value.Node.root)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		truth, e := b.state.manager.MkTrue()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		fact := value.Node.owner
		if value.Node.root.owner != truth {
			falsity, e := b.state.manager.MkFalse()
			if e != nil || value.Node.root.owner != falsity {
				if e != nil {
					return ast.ASTHandle{}, e
				}
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			fact, e = b.state.manager.MkNot(value.Node.owner)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		equalityFact, e := b.state.manager.GetProofFact(first)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rewrite, e := b.state.manager.MkProofRewrite(equalityFact, fact)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofModusPonens(first, rewrite)
	case *SMTDisequalityConflictJustification:
		proof, err = b.disequalityConflictProof(value)
	case *SMTEqualityConflictJustification:
		if value.First == nil || value.Second == nil || value.First.root == nil || value.Second.root == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		parents := make([]ast.ASTHandle, 0, 3)
		if value.First != value.First.root {
			first, e := b.equalityProof(value.First, value.First.root)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			first, e = b.state.manager.MkProofSymmetry(first)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			parents = append(parents, first)
		}
		direct, e := b.equalityJustificationProof(value.First, value.Second, value.Equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		parents = append(parents, direct)
		if value.Second != value.Second.root {
			second, e := b.equalityProof(value.Second, value.Second.root)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			parents = append(parents, second)
		}
		joined, e := b.state.manager.MkProofTransitivityStar(parents, value.First.root.owner, value.Second.root.owner)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equality, e := b.state.manager.MkEq(value.First.root.owner, value.Second.root.owner)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		falsity, e := b.state.manager.MkFalse()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rewrite, e := b.state.manager.MkProofRewrite(equality, falsity)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofModusPonens(joined, rewrite)
	case *SMTModusPonensIffJustification:
		if value.First == nil || value.Second == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		equality, e := b.equalityProof(value.First, value.Second)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		variable, found := b.state.BoolVar(value.First.owner)
		if !found || variable < SMTFirstBoolVar || int(NewSMTLiteral(variable, false).Index()) >= len(b.state.assignment) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		literal := NewSMTLiteral(variable, b.state.assignment[NewSMTLiteral(variable, false).Index()] == lbool.False)
		antecedent, e := b.literalProof(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equalityFact, e := b.state.manager.GetProofFact(equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		literalFact, e := b.state.manager.GetProofFact(antecedent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := b.state.manager.GetArg(equalityFact, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if right == literalFact {
			equality, e = b.state.manager.MkProofSymmetry(equality)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			equalityFact, e = b.state.manager.GetProofFact(equality)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		if literal.Sign() {
			left, e := b.state.manager.GetArg(equalityFact, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			negated, e := b.state.manager.GetArg(literalFact, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if left != negated {
				equality, e = b.state.manager.MkProofSymmetry(equality)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				equalityFact, e = b.state.manager.GetProofFact(equality)
				if e != nil {
					return ast.ASTHandle{}, e
				}
			}
			right, e = b.state.manager.GetArg(equalityFact, 1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			negatedRight, e := b.state.manager.MkNot(right)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			equality, e = b.state.manager.MkProofCongruence(literalFact, negatedRight, []ast.ASTHandle{equality})
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		proof, err = b.state.manager.MkProofModusPonens(antecedent, equality)
	case *SMTExtTheoryPropagationJustification:
		parents, e := b.literalProofs(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for _, pair := range value.Equalities {
			equality, e := b.equalityProof(pair.First, pair.Second)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			parents = append(parents, equality)
		}
		fact, e := b.literalExpression(value.Consequent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, parents, value.Parameters())
	case *SMTExtTheoryConflictJustification:
		parents, e := b.literalProofs(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for _, pair := range value.Equalities {
			equality, e := b.equalityProof(pair.First, pair.Second)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			parents = append(parents, equality)
		}
		fact, e := b.state.manager.MkFalse()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, parents, value.Parameters())
	case *SMTExtTheoryEqualityPropagationJustification:
		parents, e := b.literalProofs(value.Literals)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for _, pair := range value.Equalities {
			equality, e := b.equalityProof(pair.First, pair.Second)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			parents = append(parents, equality)
		}
		fact, e := b.state.manager.MkEq(value.Left.owner, value.Right.owner)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, parents, value.Parameters())
	case *SMTTheoryLemmaJustification:
		expressions, signs, e := value.Expressions()
		if e != nil || len(expressions) != len(signs) {
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		literals := make([]ast.ASTHandle, len(expressions))
		for i, expression := range expressions {
			literals[i] = expression
			if signs[i] {
				literals[i], e = b.state.manager.MkNot(expression)
				if e != nil {
					return ast.ASTHandle{}, e
				}
			}
		}
		var fact ast.ASTHandle
		if len(literals) == 1 {
			fact = literals[0]
		} else {
			fact, e = b.state.manager.MkOrArgs(literals)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		proof, err = b.state.manager.MkProofTheoryLemma(ast.FamilyID(value.theory), fact, nil, value.parameters.Parameters())
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err == nil {
		b.objects[object] = proof
	}
	return proof, err
}

func (b *smtConflictProofBuilder) dynamicAckProof(justification *smtDynamicAckJustification) (ast.ASTHandle, error) {
	if justification == nil || justification.first.IsNull() || justification.second.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	count, err := b.state.manager.GetNumArgs(justification.first)
	if err != nil || count == 0 {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	otherCount, err := b.state.manager.GetNumArgs(justification.second)
	if err != nil || otherCount != count {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	parents := make([]ast.ASTHandle, 0, count)
	literals := make([]ast.ASTHandle, 0, count+1)
	seen := make(map[ast.ASTHandle]struct{}, count)
	for index := uint32(0); index < count; index++ {
		first, e := b.state.manager.GetArg(justification.first, index)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		second, e := b.state.manager.GetArg(justification.second, index)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if first == second {
			continue
		}
		equality, e := b.state.manager.MkEq(first, second)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negated, e := b.state.manager.MkNot(equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if _, found := seen[negated]; found {
			continue
		}
		seen[negated] = struct{}{}
		literals = append(literals, negated)
		hypothesis, e := b.state.manager.MkProofHypothesis(equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		parents = append(parents, hypothesis)
	}
	if len(parents) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	congruence, err := b.state.manager.MkProofCongruence(justification.first, justification.second, parents)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := b.state.manager.MkEq(justification.first, justification.second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negatedConclusion, err := b.state.manager.MkNot(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negatedHypothesis, err := b.state.manager.MkProofHypothesis(negatedConclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	contradiction, err := b.state.manager.MkProofUnitResolution([]ast.ASTHandle{congruence, negatedHypothesis})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literals = append(literals, conclusion)
	lemma, err := b.state.manager.MkOrArgs(literals)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.state.manager.MkProofLemma(contradiction, lemma)
}

// The Go equality engine detects explicit disequalities before merging roots.
// Reconstruct the equality of the disequality literal's original operands;
// conflictProof then resolves it against that literal, as in the source's
// equality propagation followed by Boolean conflict resolution.
func (b *smtConflictProofBuilder) disequalityConflictProof(value *SMTDisequalityConflictJustification) (ast.ASTHandle, error) {
	key, err := smtCheckerPair(value.First.root, value.Second.root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literal, found := b.state.disequalityReasons[key]
	if !found || !literal.Sign() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	expression, err := b.state.BoolExpression(literal.Var())
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := b.state.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := b.state.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	first, second := b.state.context.enodes[left], b.state.context.enodes[right]
	if first == nil || second == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	start, end := first, second
	reverse := first.root != value.First.root
	if reverse {
		start, end = second, first
	}
	parents := make([]ast.ASTHandle, 0, 3)
	if start != value.First {
		p, err := b.equalityProof(start, value.First)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		parents = append(parents, p)
	}
	p, err := b.equalityJustificationProof(value.First, value.Second, value.Equality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parents = append(parents, p)
	if value.Second != end {
		p, err := b.equalityProof(value.Second, end)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		parents = append(parents, p)
	}
	p, err = b.state.manager.MkProofTransitivityStar(parents, start.owner, end.owner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if reverse {
		return b.state.manager.MkProofSymmetry(p)
	}
	return p, nil
}

func (r *SMTConflictResolver) conflictProof(conflict SMTBooleanJustification, notLiteral SMTLiteral, lemma []SMTLiteral) (ast.ASTHandle, error) {
	proofs, err := r.state.manager.ProofsEnabled()
	if err != nil || !proofs {
		return ast.ASTHandle{}, err
	}
	builder := &smtConflictProofBuilder{state: r.state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
	for _, literal := range lemma {
		builder.marked[literal.Var()] = struct{}{}
	}
	consequent := SMTFalseLiteral
	if notLiteral != SMTNullLiteral {
		consequent = notLiteral.Negated()
	}
	conflictProof, err := builder.booleanProof(consequent, conflict)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if notLiteral != SMTNullLiteral {
		other, err := builder.literalProof(notLiteral)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		conflictProof, err = r.state.manager.MkProofUnitResolution([]ast.ASTHandle{other, conflictProof})
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if len(lemma) == 0 {
		return conflictProof, nil
	}
	fact, err := builder.theoryFact(lemma)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.state.manager.MkProofLemma(conflictProof, fact)
}
