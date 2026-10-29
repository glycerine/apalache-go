package smtcore

// Source: src/smt/theory_bv.cpp:353-425,1506-1552.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source borrows the theory and variable slots, reading current assignment
// polarities when antecedents/proofs are requested within their live scope.
type smtBVFixedEqualityJustification struct {
	SMTJustificationBase
	theory        *SMTBitVectorTheory
	first, second SMTTheoryVar
}

func (*smtBVFixedEqualityJustification) Name() string              { return "bv-fixed-eq" }
func (j *smtBVFixedEqualityJustification) FromTheory() SMTTheoryID { return j.theory.FamilyID() }

func (j *smtBVFixedEqualityJustification) trueBit(literal SMTLiteral) (SMTLiteral, error) {
	s, err := j.theory.State()
	if err != nil {
		return SMTNullLiteral, err
	}
	value, err := s.Assignment(literal)
	if err != nil {
		return SMTNullLiteral, err
	}
	if value != lbool.True {
		literal = literal.Negated()
	}
	return literal, nil
}

func (j *smtBVFixedEqualityJustification) MarkAntecedents(marker SMTAntecedentMarker) error {
	if marker == nil {
		return ast.ErrASTHandle
	}
	for _, variable := range []SMTTheoryVar{j.first, j.second} {
		if _, err := j.theory.Enode(variable); err != nil {
			return err
		}
		for _, literal := range j.theory.bitLiterals[variable] {
			if literal.Var() == SMTTrueBoolVar {
				continue
			}
			truth, err := j.trueBit(literal)
			if err != nil {
				return err
			}
			if err = marker.MarkLiteral(truth); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *smtConflictProofBuilder) bvFixedEqualityProof(j *smtBVFixedEqualityJustification) (ast.ASTHandle, error) {
	first, err := j.theory.Enode(j.first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := j.theory.Enode(j.second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, right := j.theory.bitLiterals[j.first], j.theory.bitLiterals[j.second]
	if len(left) != len(right) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	parents := make([]ast.ASTHandle, 0, 2*len(left))
	visited := true
	for index := range left {
		for _, literal := range []SMTLiteral{left[index], right[index]} {
			if literal.Var() == SMTTrueBoolVar {
				continue
			}
			truth, err := j.trueBit(literal)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			proof, err := b.literalProof(truth)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if proof.IsNull() {
				visited = false
			} else {
				parents = append(parents, proof)
			}
		}
	}
	if !visited {
		return ast.ASTHandle{}, nil
	}
	fact, err := b.state.mkEqualityAtom(first.owner, second.owner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.state.manager.MkProofTheoryLemma(ast.FamilyID(j.FromTheory()), fact, parents, nil)
}

// Source region storage borrows the two enodes until the logical scope pops.
type smtBVBitEqualityJustification struct {
	SMTJustificationBase
	first, second          *SMTEnode
	theory                 SMTTheoryID
	consequent, antecedent SMTLiteral
}

func (*smtBVBitEqualityJustification) Name() string              { return "bv-bit-eq" }
func (j *smtBVBitEqualityJustification) FromTheory() SMTTheoryID { return j.theory }
func (j *smtBVBitEqualityJustification) MarkAntecedents(marker SMTAntecedentMarker) error {
	if marker == nil {
		return ast.ErrASTHandle
	}
	if err := marker.MarkEquality(j.first, j.second); err != nil {
		return err
	}
	if j.antecedent.Var() != SMTTrueBoolVar {
		return marker.MarkLiteral(j.antecedent)
	}
	return nil
}

func (b *smtConflictProofBuilder) bvBitEqualityProof(j *smtBVBitEqualityJustification) (ast.ASTHandle, error) {
	parents := make([]ast.ASTHandle, 0, 2)
	visited := true
	equality, err := b.equalityProof(j.first, j.second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if equality.IsNull() {
		visited = false
	} else {
		parents = append(parents, equality)
	}
	if j.antecedent.Var() != SMTTrueBoolVar {
		antecedent, err := b.literalProof(j.antecedent)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if antecedent.IsNull() {
			visited = false
		} else {
			parents = append(parents, antecedent)
		}
	}
	if !visited {
		return ast.ASTHandle{}, nil
	}
	fact, err := b.literalExpression(j.consequent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return b.state.manager.MkProofTheoryLemma(ast.FamilyID(j.theory), fact, parents, nil)
}
