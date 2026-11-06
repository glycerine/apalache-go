package rel

// Source: check_relation wrapping the table_relation plugin registered for
// check_table. This concrete composition checks the selected table pair and
// independently checks its whole-relation formula.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

// FormulaCheckedFinitePair owns both layers of the source composition:
// FiniteCheckedPair is the check_table relation storage and formula is the
// check_relation oracle over an independent Hash table relation.
type FormulaCheckedFinitePair struct {
	pair    *FiniteCheckedPair
	formula *FormulaCheckedTable
	retired []*FormulaCheckedTable
	closed  bool
}

// NewFormulaCheckedFinitePair transfers pair only on success. On every error
// the caller still owns pair; all formula-side construction is retired here.
func NewFormulaCheckedFinitePair(manager *RelationManager, sorts []ast.ASTHandle, pair *FiniteCheckedPair, config smtcore.SMTSolverConfig) (_ *FormulaCheckedFinitePair, resultErr error) {
	if manager == nil || pair == nil || pair.open() != nil {
		return nil, ErrRelationVerification
	}
	pairSignature, err := pair.Signature()
	if err != nil {
		return nil, err
	}
	expected, err := tableRelationSignature(manager, sorts)
	if err != nil || !pairSignature.equal(expected) {
		return nil, errors.Join(err, ErrRelationVerification)
	}
	relation, err := NewTableRelation(manager, sorts)
	if err != nil {
		return nil, err
	}
	facts, err := pair.Facts()
	if err != nil {
		return nil, errors.Join(err, relation.Close())
	}
	for _, fact := range facts {
		if _, err = relation.AddTableFact(fact); err != nil {
			return nil, errors.Join(err, relation.Close())
		}
	}
	formula, err := NewFormulaCheckedTable(relation, config)
	if err != nil {
		return nil, errors.Join(err, relation.Close())
	}
	result := &FormulaCheckedFinitePair{pair: pair, formula: formula}
	if err = result.verify("construction"); err != nil {
		result.pair = nil
		return nil, finishFormulaCheckedFinitePairConstruction(err, result)
	}
	return result, nil
}

func adoptFormulaCheckedFinitePair(pair *FiniteCheckedPair, formula *FormulaCheckedTable) (_ *FormulaCheckedFinitePair, resultErr error) {
	result := &FormulaCheckedFinitePair{pair: pair, formula: formula}
	if err := result.verify("construction"); err != nil {
		return nil, finishFormulaCheckedFinitePairConstruction(err, result)
	}
	return result, nil
}

func (t *FormulaCheckedFinitePair) open() error {
	if t == nil || t.closed || t.pair == nil || t.formula == nil || t.pair.open() != nil || t.formula.open() != nil {
		return ErrRelationVerification
	}
	return nil
}

func (t *FormulaCheckedFinitePair) verify(objective string) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.pair.verify(objective + " table pair"); err != nil {
		return err
	}
	a, err := t.pair.Facts()
	if err != nil {
		return err
	}
	b, err := t.formula.Facts()
	if err != nil {
		return err
	}
	if len(a) != len(b) {
		return fmt.Errorf("%w: %s formula row counts differ", ErrRelationVerification, objective)
	}
	for _, fact := range a {
		found, err := t.formula.Contains(fact)
		if err != nil || !found {
			return errors.Join(err, fmt.Errorf("%w: %s formula misses %v", ErrRelationVerification, objective, fact))
		}
	}
	for _, fact := range b {
		found, err := t.pair.Contains(fact)
		if err != nil || !found {
			return errors.Join(err, fmt.Errorf("%w: %s table pair misses %v", ErrRelationVerification, objective, fact))
		}
	}
	return nil
}

func (t *FormulaCheckedFinitePair) Signature() (Signature, error) {
	if err := t.verify("signature"); err != nil {
		return Signature{}, err
	}
	return t.pair.Signature()
}

func (t *FormulaCheckedFinitePair) Facts() ([]Fact, error) {
	if err := t.verify("facts"); err != nil {
		return nil, err
	}
	return t.pair.Facts()
}

func (t *FormulaCheckedFinitePair) Add(fact Fact) (bool, error) {
	if err := t.verify("add input"); err != nil {
		return false, err
	}
	a, err := t.pair.Add(fact)
	if err != nil {
		return false, err
	}
	b, err := t.formula.Add(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, ErrRelationVerification
	}
	return a, t.verify("add")
}

func (t *FormulaCheckedFinitePair) Contains(fact Fact) (bool, error) {
	if err := t.verify("contains input"); err != nil {
		return false, err
	}
	a, err := t.pair.Contains(fact)
	if err != nil {
		return false, err
	}
	b, err := t.formula.Contains(fact)
	if err != nil || a != b {
		return false, errors.Join(err, ErrRelationVerification)
	}
	return a, nil
}

func (t *FormulaCheckedFinitePair) Len() (int, error) {
	if err := t.verify("length"); err != nil {
		return 0, err
	}
	return t.pair.Len()
}

func (t *FormulaCheckedFinitePair) Reset() error {
	if err := t.verify("reset input"); err != nil {
		return err
	}
	if err := t.pair.Reset(); err != nil {
		return err
	}
	if err := t.formula.Reset(); err != nil {
		return err
	}
	return t.verify("reset")
}

func (t *FormulaCheckedFinitePair) CloneFinite() (FiniteTable, error) {
	if err := t.verify("clone input"); err != nil {
		return nil, err
	}
	pair, err := t.pair.CloneFinite()
	if err != nil {
		return nil, err
	}
	formula, err := t.formula.Clone()
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair.(*FiniteCheckedPair), formula)
}

func (t *FormulaCheckedFinitePair) SizeEstimateRows() (uint32, error) {
	if err := t.verify("size"); err != nil {
		return 0, err
	}
	return t.pair.SizeEstimateRows()
}

func (t *FormulaCheckedFinitePair) SizeEstimateBytes() (uint32, error) {
	if err := t.verify("size"); err != nil {
		return 0, err
	}
	return t.pair.SizeEstimateBytes()
}

func (t *FormulaCheckedFinitePair) EmptyLike() (*FormulaCheckedFinitePair, error) {
	if err := t.verify("empty input"); err != nil {
		return nil, err
	}
	pair, err := t.pair.EmptyLike()
	if err != nil {
		return nil, err
	}
	formula, err := emptyFormulaCheckedLike(t.formula)
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair, formula)
}

func (t *FormulaCheckedFinitePair) ConcreteKinds() (primary, checker string, formula bool, err error) {
	if err = t.verify("kinds"); err != nil {
		return "", "", false, err
	}
	primary, checker, err = t.pair.ConcreteKinds()
	return primary, checker, true, err
}

func (t *FormulaCheckedFinitePair) FilterEqual(value Element, column int) error {
	if err := t.verify("filter equal input"); err != nil {
		return err
	}
	if err := t.pair.FilterEqual(value, column); err != nil {
		return err
	}
	if err := formulaCheckedFilterEqual(t.formula, value, column); err != nil {
		return err
	}
	return t.verify("filter equal")
}

func (t *FormulaCheckedFinitePair) FilterIdentical(columns []int) error {
	if err := t.verify("filter identical input"); err != nil {
		return err
	}
	if err := t.pair.FilterIdentical(columns); err != nil {
		return err
	}
	if err := t.formula.FilterIdentical(columns); err != nil {
		return err
	}
	return t.verify("filter identical")
}

func (t *FormulaCheckedFinitePair) FilterInterpreted(manager *RelationManager, sorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, variables map[uint32]ast.ASTHandle) error {
	if err := t.verify("filter interpreted input"); err != nil {
		return err
	}
	if err := t.pair.FilterInterpreted(manager, sorts, theory, conditions, columns, variables); err != nil {
		return err
	}
	if err := formulaCheckedFilterInterpretedBound(t.formula, theory, conditions, columns, variables); err != nil {
		return err
	}
	return t.verify("filter interpreted")
}

func (t *FormulaCheckedFinitePair) Close() (result error) {
	if t == nil || t.closed {
		return nil
	}
	for len(t.retired) != 0 {
		index := len(t.retired) - 1
		if err := t.retired[index].Close(); err != nil {
			return errors.Join(result, err)
		}
		t.retired[index] = nil
		t.retired = t.retired[:index]
	}
	if t.pair != nil {
		if err := t.pair.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			t.pair = nil
		}
	}
	if t.formula != nil {
		if err := t.formula.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			t.formula = nil
		}
	}
	if result == nil {
		t.closed = true
	}
	return result
}

var _ FiniteTable = (*FormulaCheckedFinitePair)(nil)
