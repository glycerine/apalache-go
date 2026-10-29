package smtcore

// Source: src/smt/theory_bv.cpp:75-137,210-220,263-297,1163-1185;
// theory_bv.h var_pos_occ and bit_atom.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Entries are stored in insertion order and traversed backwards to represent
// the source linked-list head. Repeated bits retain their distinct positions.
type smtBVBitOccurrence struct {
	variable SMTTheoryVar
	index    uint32
}

type smtBVBitOccurrenceTrail struct{ boolean int32 }

func (t *SMTBitVectorTheory) CreateVariable(node *SMTEnode) (SMTTheoryVar, error) {
	variable, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return variable, err
	}
	t.privateFind = append(t.privateFind, variable)
	t.privateSize = append(t.privateSize, 1)
	t.privateNext = append(t.privateNext, variable)
	t.privateTrail = append(t.privateTrail, smtBVPrivateTrail{kind: smtBVNewVariable})
	t.bitLiterals = append(t.bitLiterals, nil)
	t.watchedPosition = append(t.watchedPosition, 0)
	t.zeroOneBits = append(t.zeroOneBits, nil)
	return variable, nil
}

// Source get_var/mk_bits (theory_bv.cpp:49-63,160-167) is distinct
// from apply_sort_cnstr: it creates indexed native atoms even if sort attachment
// was suppressed or the approximation flag was already set. It does not find_wpos.
func (t *SMTBitVectorTheory) nativeVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if node == nil {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	variable := node.TheoryVar(t.FamilyID())
	if variable != SMTNullTheoryVar {
		return variable, nil
	}
	width, err := t.state.manager.BitVectorSize(node.owner)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	variable, err = t.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	bits, err := t.symbolicBits(node.owner, width)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.publishBitOwner(node.owner, bits); err != nil {
		return SMTNullTheoryVar, err
	}
	for _, bit := range bits {
		if err = t.state.Internalize(bit, true); err != nil {
			return SMTNullTheoryVar, err
		}
		boolean, found := t.state.BoolVar(bit)
		if !found {
			return SMTNullTheoryVar, ast.ErrASTDomain
		}
		t.bitLiterals[variable] = append(t.bitLiterals[variable], NewSMTLiteral(boolean, false))
	}
	return variable, nil
}

func (t *SMTBitVectorTheory) addBitOccurrence(boolean int32, variable SMTTheoryVar, index uint32) error {
	if boolean == SMTTrueBoolVar {
		return nil
	}
	if boolean < SMTFirstBoolVar || variable < 0 || int(variable) >= len(t.variables) {
		return ast.ErrASTDomain
	}
	occurrences, exists := t.bitAtoms[boolean]
	// Reentrant bit2bool internalization can encounter its own already
	// published occurrence. Source does not register the same pair twice.
	for _, entry := range occurrences {
		if entry.variable == variable && entry.index == index {
			return nil
		}
	}
	if !exists {
		if err := bvAssert(293, "b->m_occs == 0", func() (bool, error) { return len(occurrences) == 0, nil }); err != nil {
			return err
		}
		t.bitAtoms[boolean] = nil
		t.bitAtomTrail = append(t.bitAtomTrail, boolean)
		t.privateTrail = append(t.privateTrail, smtBVPrivateTrail{kind: smtBVNewAtom, boolean: boolean})
	}
	t.bitAtoms[boolean] = append(occurrences, smtBVBitOccurrence{variable, index})
	t.bitOccurrenceTrail = append(t.bitOccurrenceTrail, smtBVBitOccurrenceTrail{boolean})
	t.privateTrail = append(t.privateTrail, smtBVPrivateTrail{kind: smtBVNewOccurrence, boolean: boolean})
	return nil
}

// Source add_bit publishes the literal before its occurrence. Each caller
// reaches this immediately after actual Boolean internalization of that bit.
func (t *SMTBitVectorTheory) registerBitLiteral(expression ast.ASTHandle, index uint32, literal SMTLiteral) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	node := s.context.enodes[expression]
	if node == nil {
		return ast.ErrASTDomain
	}
	variable := node.TheoryVar(t.FamilyID())
	if variable < 0 || int(variable) >= len(t.bitLiterals) {
		return ast.ErrASTDomain
	}
	bits := t.bitLiterals[variable]
	if uint64(index) < uint64(len(bits)) {
		if bits[index] != literal {
			return ast.ErrASTDomain
		}
		return nil
	}
	if uint64(index) != uint64(len(bits)) {
		return ast.ErrASTDomain
	}
	t.bitLiterals[variable] = append(bits, literal)
	if literal.Var() == SMTTrueBoolVar {
		if err = bvAssert(244, "m_bits[v][idx] == true_literal || m_bits[v][idx] == false_literal", func() (bool, error) {
			return literal == SMTTrueLiteral || literal == SMTFalseLiteral, nil
		}); err != nil {
			return err
		}
		t.zeroOneBits[variable] = append(t.zeroOneBits[variable], smtBVZeroOneBit{variable: variable, index: index, isTrue: literal == SMTTrueLiteral})
		return nil
	}
	data, err := s.BooleanData(literal.Var())
	if err != nil {
		return err
	}
	theoryID := data.Theory()
	if theoryID == t.FamilyID() {
		if err = bvAssert(280, "a && a->is_bit()", func() (bool, error) {
			_, exists := t.bitAtoms[literal.Var()]
			return exists, nil
		}); err != nil {
			return err
		}
		if _, exists := t.bitAtoms[literal.Var()]; !exists {
			return ast.ErrASTDomain
		}
	} else {
		if err = bvAssert(287, "th_id == null_theory_id", func() (bool, error) { return theoryID == SMTNullTheoryID, nil }); err != nil {
			return err
		}
		if theoryID != SMTNullTheoryID {
			return ast.ErrASTDomain
		}
		if err = s.SetVariableTheory(literal.Var(), t.FamilyID()); err != nil {
			return err
		}
		if err = bvAssert(289, "ctx.get_var_theory(l.var()) == get_id()", func() (bool, error) {
			data, e := s.BooleanData(literal.Var())
			return e == nil && data.Theory() == t.FamilyID(), e
		}); err != nil {
			return err
		}
	}
	if err = t.findNewBitDisequalities(variable, index, literal); err != nil {
		return err
	}
	return t.addBitOccurrence(literal.Var(), variable, index)
}

// Source display_atoms orders bit atoms by Boolean variable, while each atom
// prints its newest occurrence first. Empty surviving atoms still print a row.
func (t *SMTBitVectorTheory) displayBitAtoms(out io.Writer) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err = smtWriteTheoryText(out, "atoms:\n"); err != nil {
		return err
	}
	for boolean := int32(0); int(boolean) < len(s.boolExpr); boolean++ {
		entries, exists := t.bitAtoms[boolean]
		if !exists {
			continue
		}
		id, err := s.manager.GetASTID(s.boolExpr[boolean])
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%d ->", id)); err != nil {
			return err
		}
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			node, err := t.Enode(entry.variable)
			if err != nil {
				return err
			}
			id, err := node.OwnerID()
			if err != nil {
				return err
			}
			if err = smtWriteTheoryText(out, fmt.Sprintf(" #%d[%d]", id, entry.index)); err != nil {
				return err
			}
		}
		if err = smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// Source mk_bit2bool first internalizes its argument. That initialization may
// create this very Boolean atom; it must not be allocated ahead of earlier bits.
func (t *SMTBitVectorTheory) internalizeBitProjection(expression ast.ASTHandle) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	if err = bvAssert(77, "!ctx.b_internalized(n)", func() (bool, error) {
		_, found := s.BoolVar(expression)
		return !found, nil
	}); err != nil {
		return false, err
	}
	owner, err := s.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	missing := s.context.enodes[owner] == nil
	if err = s.Internalize(owner, false); err != nil {
		return false, err
	}
	if missing {
		if err = bvAssert(87, "ctx.e_internalized(first_arg)", func() (bool, error) { return s.context.enodes[owner] != nil, nil }); err != nil {
			return false, err
		}
	}
	node := s.context.enodes[owner]
	if node == nil {
		return false, ast.ErrASTDomain
	}
	missingVariable := node.TheoryVar(t.FamilyID()) == SMTNullTheoryVar
	if missingVariable {
		approximate, err := t.approximateTerm(owner)
		if err != nil {
			return false, err
		}
		if approximate {
			variable, found := s.BoolVar(expression)
			if !found {
				variable, err = s.MkBoolVar(expression)
				if err != nil {
					return false, err
				}
			}
			if err = s.SetVariableTheory(variable, t.FamilyID()); err != nil {
				return false, err
			}
			return true, nil // Preserve the existing approximate-term unknown result.
		}
		// Native get_var handles ITE enodes whose sort constraint was suppressed.
		if _, err = t.CreateVariable(node); err != nil {
			return false, err
		}
		bits, err := t.bitsFor(owner)
		if err != nil {
			return false, err
		}
		for i, bit := range bits {
			if err = s.Internalize(bit, true); err != nil {
				return false, err
			}
			literal, err := s.Literal(bit)
			if err != nil {
				return false, err
			}
			if err = t.registerBitLiteral(owner, uint32(i), literal); err != nil {
				return false, err
			}
		}
	}
	if missingVariable {
		if err = bvAssert(124, "ctx.b_internalized(n)", func() (bool, error) {
			_, found := s.BoolVar(expression)
			return found, nil
		}); err != nil {
			return false, err
		}
	}
	if _, found := s.BoolVar(expression); found {
		return true, nil
	}
	bits, err := t.bitsFor(owner)
	if err != nil {
		return false, err
	}
	index, err := t.bit2BoolIndex(expression)
	if err != nil {
		return false, err
	}
	if uint64(index) >= uint64(len(bits)) {
		return false, ast.ErrASTDomain
	}
	op, own, err := t.operation(owner)
	if err != nil {
		return false, err
	}
	if missing && own && op == smtBVOpNumeral {
		// Source creates all numeral projections in increasing index order on
		// this first visit; subsequent visits create only their requested atom.
		for i := range bits {
			atom, err := s.manager.MkBitVectorBit2Bool(uint32(i), owner)
			if err != nil {
				return false, err
			}
			if err = s.Internalize(atom, true); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if err = bvAssert(127, "v_arg != null_theory_var", func() (bool, error) {
		return node.TheoryVar(t.FamilyID()) != SMTNullTheoryVar, nil
	}); err != nil {
		return false, err
	}
	variable, err := s.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = s.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	if err = bvAssert(134, "a->m_occs == 0", func() (bool, error) {
		entries, exists := t.bitAtoms[variable]
		return !exists || len(entries) == 0, nil
	}); err != nil {
		return false, err
	}
	if err = t.addBitOccurrence(variable, node.TheoryVar(t.FamilyID()), index); err != nil {
		return false, err
	}
	// Native mk_bit2bool publishes an atom/occurrence only. Canonical
	// bits and retained callers supply their own native equality axioms.
	return true, nil
}

// Source find_new_diseq_axioms consumes the old linked head before add_bit
// prepends its own occurrence. Constants use the separate class summary path.
func (t *SMTBitVectorTheory) findNewBitDisequalities(variable SMTTheoryVar, index uint32, literal SMTLiteral) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	width, err := s.manager.BitVectorSize(node.owner)
	if err != nil {
		return err
	}
	old := t.bitAtoms[literal.Var()]
	for i := len(old) - 1; i >= 0; i-- {
		entry := old[i]
		if entry.index != index {
			continue
		}
		if entry.variable < 0 || int(entry.variable) >= len(t.bitLiterals) || uint64(index) >= uint64(len(t.bitLiterals[entry.variable])) {
			return ast.ErrASTDomain
		}
		if t.bitLiterals[entry.variable][index] != literal.Negated() {
			continue
		}
		other, err := t.Enode(entry.variable)
		if err != nil {
			return err
		}
		otherWidth, err := s.manager.BitVectorSize(other.owner)
		if err != nil {
			return err
		}
		if width != otherWidth {
			continue
		}
		if err = t.newStaticBitDisequality(variable, entry.variable, index); err != nil {
			return err
		}
	}
	return nil
}

// Source mk_new_diseq_axiom is also called by constant-summary merging.
func (t *SMTBitVectorTheory) newStaticBitDisequality(left, right SMTTheoryVar, index uint32) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	node, err := t.Enode(left)
	if err != nil {
		return err
	}
	other, err := t.Enode(right)
	if err != nil {
		return err
	}
	if err = bvAssert(226, "m_bits[v1][idx] == ~m_bits[v2][idx]", func() (bool, error) {
		if left < 0 || right < 0 || int(left) >= len(t.bitLiterals) || int(right) >= len(t.bitLiterals) || uint64(index) >= uint64(len(t.bitLiterals[left])) || uint64(index) >= uint64(len(t.bitLiterals[right])) {
			return false, ast.ErrASTDomain
		}
		return t.bitLiterals[left][index] == t.bitLiterals[right][index].Negated(), nil
	}); err != nil {
		return err
	}
	if err = t.traceBVStaticDisequality(left, right); err != nil {
		return err
	}
	t.statistics.diseqStatic++
	equality, err := t.MkEqualityLiteral(node.owner, other.owner, true)
	if err != nil {
		return err
	}
	if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality.Negated()}); err != nil {
		return err
	}
	if s.relevancy != nil && s.relevancy.enabled {
		expression, err := s.BoolExpression(equality.Var())
		if err != nil {
			return err
		}
		if err = s.relevancy.AddPairDependency(node.owner, other.owner, expression); err != nil {
			return err
		}
	}
	return nil
}
