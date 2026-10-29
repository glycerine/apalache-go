package smtcore

// Source: theory_array::new_diseq_eh and theory_array_base extensionality queue.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtArrayEnodePair struct{ first, second *SMTEnode }

func (t *SMTArrayTheory) NewDisequality(left, right SMTTheoryVar) error {
	first, err := t.Enode(left)
	if err != nil {
		return err
	}
	second, err := t.Enode(right)
	if err != nil {
		return err
	}
	first, err = t.Enode(SMTTheoryVar(t.arrayUnion.Find(uint32(left))))
	if err != nil {
		return err
	}
	second, err = t.Enode(SMTTheoryVar(t.arrayUnion.Find(uint32(right))))
	if err != nil {
		return err
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	sort, err := s.manager.GetSort(first.owner)
	if err != nil {
		return err
	}
	family, err := s.manager.GetFamilyID(sort)
	if err != nil {
		return err
	}
	secondSort, err := s.manager.GetSort(second.owner)
	if err != nil {
		return err
	}
	secondFamily, err := s.manager.GetFamilyID(secondSort)
	if err != nil {
		return err
	}
	if family == t.FamilyID() {
		if err = arraySourceAssert(307, "m_var_data[v2]->m_is_array", func() bool { return secondFamily == t.FamilyID() }); err != nil {
			return err
		}
	}
	if family != t.FamilyID() || secondFamily != t.FamilyID() || !t.extensional {
		return err
	}
	if err = t.emitSimpleArrayTrace("ext", "new_diseq_eh", 308, nil, func(out io.Writer) error {
		if _, err := io.WriteString(out, "extensionality:\n"); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(s.manager, out, first.owner); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(s.manager, out, second.owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	return t.instantiateArrayExtensionality(first, second)
}

func (t *SMTArrayTheory) instantiateArrayExtensionality(first, second *SMTEnode) error {
	if !t.extensional {
		return nil
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	leftSort, err := state.manager.GetSort(first.owner)
	if err != nil {
		return err
	}
	rightSort, err := state.manager.GetSort(second.owner)
	if err != nil {
		return err
	}
	leftFamily, err := state.manager.GetFamilyID(leftSort)
	if err != nil {
		return err
	}
	rightFamily, err := state.manager.GetFamilyID(rightSort)
	if err != nil {
		return err
	}
	if err = arraySourceAssert(227, "is_array_sort(a1)", func() bool { return leftFamily == t.FamilyID() }); err != nil {
		return err
	}
	if err = arraySourceAssert(228, "is_array_sort(a2)", func() bool { return rightFamily == t.FamilyID() }); err != nil {
		return err
	}
	if leftFamily != t.FamilyID() || rightFamily != t.FamilyID() {
		return ast.ErrASTDomain
	}
	if err := t.emitSimpleArrayTrace("array", "instantiate_extensionality", 226, nil, func(out io.Writer) error {
		leftID, err := first.OwnerID()
		if err != nil {
			return err
		}
		rightID, err := second.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "extensionality: #%d #%d\n", leftID, rightID)
		return err
	}); err != nil {
		return err
	}
	leftID, err := first.OwnerID()
	if err != nil {
		return err
	}
	rightID, err := second.OwnerID()
	if err != nil {
		return err
	}
	if leftID > rightID {
		first, second = second, first
	}
	// fingerprint_set::insert checks the original ordered node pair before
	// checking current roots; stored pairs remain snapshots across merges.
	if t.extensionalityFingerprints[smtArrayEnodePair{first, second}] {
		return nil
	}
	pair := smtArrayEnodePair{first.root, second.root}
	if t.extensionalityFingerprints[pair] {
		return nil
	}
	t.extensionalityFingerprints[pair] = true
	t.extensionalityTrail = append(t.extensionalityTrail, pair)
	already, err := t.alreadyDisequalReads(first, second)
	if err != nil || already {
		return err
	}
	t.extensionalityTodo = append(t.extensionalityTodo, smtArrayEnodePair{first, second})
	t.statistics.extensionality++
	return nil
}

func (t *SMTArrayTheory) alreadyDisequalReads(first, second *SMTEnode) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	first, second = first.root, second.root
	if first.classSize > second.classSize {
		first, second = second, first
	}
	var reads []*SMTEnode
	for _, parent := range first.parents {
		op, array, err := t.arrayOperation(parent.owner)
		if err != nil {
			return false, err
		}
		if !array || op != ast.ASTArrayOpSelect || !parent.IsCongruenceRoot() || len(parent.arguments) < 2 || parent.arguments[0].root != first || s.relevancy != nil && !s.relevancy.IsRelevant(parent.owner) {
			continue
		}
		reads = append(reads, parent)
	}
	for _, parent := range second.parents {
		op, array, err := t.arrayOperation(parent.owner)
		if err != nil {
			return false, err
		}
		if !array || op != ast.ASTArrayOpSelect || !parent.IsCongruenceRoot() || len(parent.arguments) < 2 || parent.arguments[0].root != second || s.relevancy != nil && !s.relevancy.IsRelevant(parent.owner) {
			continue
		}
		for _, other := range reads {
			if len(parent.arguments) != len(other.arguments) {
				continue
			}
			same := true
			for i := 1; i < len(parent.arguments); i++ {
				if parent.arguments[i].root != other.arguments[i].root {
					same = false
					break
				}
			}
			if !same {
				continue
			}
			different, err := t.hasRelevantDisequality(parent, other)
			if err != nil {
				return false, err
			}
			if different {
				if err = t.emitArrayTrace("array_ext", "already_diseq", 281, nil, func(out io.Writer) error {
					_, err := io.WriteString(out, "selects are disequal\n")
					return err
				}); err != nil {
					return false, err
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// Source context::is_diseq: find a relevant equality assigned false modulo
// egraph roots. Boolean equality uses iff and need not create a disequality
// in the egraph's non-Boolean disequality table.
func (t *SMTArrayTheory) hasRelevantDisequality(first, second *SMTEnode) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	for variable := SMTFirstBoolVar; int(variable) < len(s.boolExpr); variable++ {
		expression := s.boolExpr[variable]
		op, basic, err := s.basicOperation(expression)
		if err != nil {
			return false, err
		}
		if !basic || op != ast.ASTOpEq && op != ast.ASTOpIff || s.relevancy != nil && !s.relevancy.IsRelevant(expression) {
			continue
		}
		value, err := s.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return false, err
		}
		if value != lbool.False {
			continue
		}
		a, err := s.manager.GetArg(expression, 0)
		if err != nil {
			return false, err
		}
		b, err := s.manager.GetArg(expression, 1)
		if err != nil {
			return false, err
		}
		left, right := s.context.enodes[a], s.context.enodes[b]
		if left == nil || right == nil {
			continue
		}
		if left.root == first.root && right.root == second.root || left.root == second.root && right.root == first.root {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTArrayTheory) CanPropagate() bool {
	return len(t.storeTodo) != 0 || len(t.readTodo) != 0 || len(t.extensionalityTodo) != 0
}

func (t *SMTArrayTheory) Propagate() error {
	for t.CanPropagate() {
		for index := 0; index < len(t.storeTodo); index++ {
			if err := t.assertStoreAxiomCore(t.storeTodo[index]); err != nil {
				return err
			}
		}
		t.storeTodo = nil
		for index := 0; index < len(t.readTodo); index++ {
			pair := t.readTodo[index]
			if err := t.assertStoreReadAxiomCore(pair.observer, pair.array); err != nil {
				return err
			}
		}
		t.readTodo = nil
		for index := 0; index < len(t.extensionalityTodo); index++ {
			pair := t.extensionalityTodo[index]
			if err := t.assertExtensionalityCore(pair.first.owner, pair.second.owner); err != nil {
				return err
			}
		}
		t.extensionalityTodo = nil
	}
	return nil
}
