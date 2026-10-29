package smtcore

// Source: theory_datatype::assert_eq_axiom and dt_eq_justification.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTDatatypeTheory) equalityAxiom(left, right ast.ASTHandle, antecedent SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	proofs, err := state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	markLiteral := func(literal SMTLiteral) error {
		if literal == SMTTrueLiteral || literal == SMTFalseLiteral {
			return nil
		}
		expression, err := state.BoolExpression(literal.Var())
		if err != nil {
			return err
		}
		return state.markRelevant(expression)
	}
	clause := func(markAntecedent bool) error {
		literal, err := t.MkEqualityLiteral(left, right, true)
		if err != nil {
			return err
		}
		if err = markLiteral(literal); err != nil {
			return err
		}
		literals := []SMTLiteral{literal}
		if antecedent != SMTNullLiteral {
			if markAntecedent {
				if err = markLiteral(antecedent); err != nil {
					return err
				}
			}
			literals = append(literals, antecedent.Negated())
		}
		return state.addTheoryAxiom(t.FamilyID(), literals)
	}
	if proofs {
		return clause(false)
	}
	if err = state.Internalize(right, false); err != nil {
		return err
	}
	lhs, rhs := state.context.enodes[left], state.context.enodes[right]
	if lhs == nil || rhs == nil {
		return ast.ErrASTDomain
	}
	if err = t.emitDatatype("datatype", "assert_eq_axiom", 100, func(out io.Writer) error {
		_, _ = io.WriteString(out, "adding axiom:\n")
		if err := t.displayDatatypeExpression(out, left); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n=\n")
		if err := t.displayDatatypeExpression(out, right); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	if antecedent == SMTNullLiteral {
		return state.assignEquality(lhs, rhs, NewSMTEqualityAxiom())
	}
	assignment, err := state.Assignment(antecedent)
	if err != nil {
		return err
	}
	if assignment != lbool.True {
		return clause(true)
	}
	owner, err := NewSMTExtTheoryEqualityPropagationJustification(
		state.manager, t.FamilyID(), []SMTLiteral{antecedent}, nil, lhs, rhs, nil)
	if err != nil {
		return err
	}
	// Source dt_eq_justification::get_from_theory returns null: this
	// equality must notify the datatype theory that originated it.
	owner.returnToOwner = true
	// Transfer before publishing the queued equality: a later merge error
	// can leave an equality edge or conflict referencing this scoped owner.
	state.ownedJustifications = append(state.ownedJustifications, owner)
	if err = t.emitDatatype("datatype", "assert_eq_axiom", 116, func(out io.Writer) error {
		_, _ = io.WriteString(out, "assigning... #")
		leftID, err := lhs.OwnerID()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "%d #", leftID)
		rightID, err := rhs.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%d\nv%d v%d\n", rightID, lhs.TheoryVar(t.FamilyID()), rhs.TheoryVar(t.FamilyID()))
		return err
	}); err != nil {
		return err
	}
	if err = t.emitDatatype("datatype_detail", "assert_eq_axiom", 118, t.Display); err != nil {
		return err
	}
	return state.assignEquality(lhs, rhs, NewSMTEqualityJustification(owner))
}
