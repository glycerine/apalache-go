package rel

// Source: src/muz/rel/check_relation.cpp/.h, concrete table-backed checking
// relation, fact insertion, emptiness, reset, grounding and equivalence.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var ErrRelationVerification = errors.New("muz/rel: operation was not verified")

// FormulaCheckedTable owns a concrete table relation and keeps the source
// check relation's expected formula. Solver services are borrowed.
type FormulaCheckedTable struct {
	table *TableRelation
	*relationFormulaChecker
	formula ast.ASTRef
	closed  bool
}

// Table returns the borrowed concrete table under verification. It is exposed
// for the fixedpoint execution context, which keeps its existing concrete
// table compiler while routing mutations through this checker.
func (r *FormulaCheckedTable) Table() *HashTable {
	if r.open() != nil {
		return nil
	}
	return r.table.Table()
}

// Facts returns the verified concrete rows.
func (r *FormulaCheckedTable) Facts() ([]Fact, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.table.Facts()
}

// AddTableFact decodes an encoded Datalog row and checks the insertion against
// the relation formula, matching check_relation::add_fact at the table boundary.
func (r *FormulaCheckedTable) AddTableFact(fact Fact) (added bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	values, err := r.table.manager.DecodeFact(r.table.sorts, fact)
	if err != nil {
		return false, err
	}
	defer func() {
		for i := len(values) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&values[i]))
		}
	}()
	handles := make([]ast.ASTHandle, len(values))
	for i := range values {
		handles[i], err = r.manager.RefGet(values[i])
		if err != nil {
			return false, err
		}
	}
	return r.AddFact(handles)
}

// NewFormulaCheckedTable transfers source ownership on success.
func NewFormulaCheckedTable(source *TableRelation, config smtcore.SMTSolverConfig) (*FormulaCheckedTable, error) {
	if source == nil || source.open() != nil || config.Params == nil {
		return nil, ErrTableRelation
	}
	m := source.manager.manager
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	formula, err := source.ToFormula()
	if err != nil {
		return nil, err
	}
	return &FormulaCheckedTable{table: source, relationFormulaChecker: &relationFormulaChecker{manager: m, config: config, sorts: source.sorts}, formula: formula}, nil
}

func (r *FormulaCheckedTable) open() error {
	if r == nil || r.closed || r.table == nil || r.table.open() != nil || r.manager == nil || r.formula == (ast.ASTRef{}) {
		return ErrTableRelation
	}
	return nil
}

// ground replaces each column variable with the source numeric-symbol constant.
func (r *relationFormulaChecker) ground(formula ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	return r.groundWithNames(formula, false)
}

func (r *relationFormulaChecker) groundWithNames(formula ast.ASTHandle, named bool) (result ast.ASTRef, resultErr error) {
	var refs []ast.ASTRef
	owned := false
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	vars := make([]ast.ASTHandle, len(r.sorts))
	for i, sort := range r.sorts {
		name := symbol.Numerical(uint32(i))
		if named {
			var err error
			name, err = r.manager.Symbols.Intern(fmt.Sprintf("x%d", i))
			if err != nil {
				return result, err
			}
		}
		value, err := r.manager.MkConst(name, sort)
		if err != nil {
			return result, err
		}
		ref, err := r.manager.NewRef(value)
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
		vars[i] = value
	}
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
	}
	if err != nil {
		return result, err
	}
	result, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	owned = true
	resultErr = subst.Substitute(formula, vars, result)
	return result, resultErr
}

func (r *relationFormulaChecker) checkEquivalent(left, right ast.ASTHandle) (resultErr error) {
	return VerifyRelationFormulasEquivalent(r.manager, r.config, left, right)
}

func (r *relationFormulaChecker) factFormula(values []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := r.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	conjuncts := make([]ast.ASTHandle, 0, len(values))
	for i, value := range values {
		variable, err := m.MkVar(uint32(i), r.sorts[i])
		if err != nil {
			return result, err
		}
		varRef, err := m.NewRef(variable)
		if err != nil {
			return result, err
		}
		refs = append(refs, varRef)
		eq, err := m.MkEq(variable, value)
		if err != nil {
			return result, err
		}
		eqRef, err := m.NewRef(eq)
		if err != nil {
			return result, err
		}
		refs = append(refs, eqRef)
		conjuncts = append(conjuncts, eq)
	}
	expression, err := m.MkAndArray(conjuncts)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(expression)
	if err != nil {
		return result, err
	}
	return result, nil
}

// AddFact updates the table and verifies its new formula against the old
// formula disjoined with the concrete fact.
func (r *FormulaCheckedTable) AddFact(values []ast.ASTHandle) (added bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	added, err := r.table.AddFact(values)
	if err != nil {
		return false, err
	}
	actual, err := r.table.ToFormula()
	if err != nil {
		return added, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&actual)) }()
	fact, err := r.factFormula(values)
	if err != nil {
		return added, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&fact)) }()
	old, err := r.manager.RefGet(r.formula)
	if err != nil {
		return added, err
	}
	factHandle, err := r.manager.RefGet(fact)
	if err != nil {
		return added, err
	}
	expected, err := r.manager.MkOrArgs([]ast.ASTHandle{old, factHandle})
	if err != nil {
		return added, err
	}
	if err = r.manager.SetRef(r.formula, expected); err != nil {
		return added, err
	}
	groundExpected, err := r.ground(expected)
	if err != nil {
		return added, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&groundExpected)) }()
	actualHandle, err := r.manager.RefGet(actual)
	if err != nil {
		return added, err
	}
	groundActual, err := r.ground(actualHandle)
	if err != nil {
		return added, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&groundActual)) }()
	a, err := r.manager.RefGet(groundExpected)
	if err != nil {
		return added, err
	}
	b, err := r.manager.RefGet(groundActual)
	if err != nil {
		return added, err
	}
	if err = r.checkEquivalent(a, b); err != nil {
		return added, fmt.Errorf("add_fact: %w", err)
	}
	return added, r.manager.SetRef(r.formula, actualHandle)
}

// AddNewFact follows the source caller contract that values are not already
// present, while still checking the concrete table update against its formula.
func (r *FormulaCheckedTable) AddNewFact(values []ast.ASTHandle) error {
	_, err := r.AddFact(values)
	return err
}

func (r *FormulaCheckedTable) ContainsFact(values []ast.ASTHandle) (found bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	found, err := r.table.ContainsFact(values)
	if err != nil {
		return false, err
	}
	formula, err := r.manager.RefGet(r.formula)
	if err != nil {
		return false, err
	}
	if !found && r.manager.IsFalse(formula) {
		return false, nil
	}
	fact, err := r.factFormula(values)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&fact)) }()
	factHandle, err := r.manager.RefGet(fact)
	if err != nil {
		return false, err
	}
	intersection, err := r.manager.MkAndArgs([]ast.ASTHandle{formula, factHandle})
	if err != nil {
		return false, err
	}
	intersectionRef, err := r.manager.NewRef(intersection)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&intersectionRef)) }()
	left, right := intersection, factHandle
	if !found {
		right, err = r.manager.MkFalse()
		if err != nil {
			return false, err
		}
	}
	if err = r.verifyGrounded(left, right, false); err != nil {
		return found, fmt.Errorf("contains fact: %w", err)
	}
	return found, nil
}

func (r *FormulaCheckedTable) Clone() (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	table, err := r.table.Clone()
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = finishFormulaCheckedTableConstruction(resultErr, result)
		}
	}()
	left, err := r.manager.RefGet(r.formula)
	if err != nil {
		return nil, err
	}
	right, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if left != right {
		if err = r.verifyGrounded(left, right, false); err != nil {
			return nil, fmt.Errorf("clone: %w", err)
		}
	}
	return result, nil
}

func (r *FormulaCheckedTable) Empty() (empty bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	if !r.table.Empty() {
		return false, nil
	}
	formula, err := r.manager.RefGet(r.formula)
	if err != nil {
		return false, err
	}
	if r.manager.IsFalse(formula) {
		return true, nil
	}
	ground, err := r.ground(formula)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&ground)) }()
	value, err := r.manager.RefGet(ground)
	if err != nil {
		return false, err
	}
	falsehood, err := r.manager.MkFalse()
	if err != nil {
		return false, err
	}
	if err = r.checkEquivalent(falsehood, value); err != nil {
		return false, fmt.Errorf("empty: %w", err)
	}
	return true, nil
}

// FastEmpty checks the underlying relation's fast emptiness result against
// the recorded formula, matching check_relation::fast_empty.
func (r *FormulaCheckedTable) FastEmpty() (empty bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	if !r.table.Empty() {
		return false, nil
	}
	formula, err := r.manager.RefGet(r.formula)
	if err != nil {
		return false, err
	}
	if r.manager.IsFalse(formula) {
		return true, nil
	}
	ground, err := r.ground(formula)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&ground)) }()
	value, err := r.manager.RefGet(ground)
	if err != nil {
		return false, err
	}
	falsehood, err := r.manager.MkFalse()
	if err != nil {
		return false, err
	}
	if err = r.checkEquivalent(falsehood, value); err != nil {
		return false, fmt.Errorf("fast_empty: %w", err)
	}
	return true, nil
}

func (r *FormulaCheckedTable) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.table.Reset(); err != nil {
		return err
	}
	value, err := r.manager.MkFalse()
	if err != nil {
		return err
	}
	return r.manager.SetRef(r.formula, value)
}

func (r *FormulaCheckedTable) Close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	if r.formula != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.formula))
	}
	if r.formula != (ast.ASTRef{}) {
		return result
	}
	if r.table != nil {
		result = errors.Join(result, r.table.Close())
		r.table = nil
	}
	r.closed, r.manager = true, nil
	return result
}

// Complement checks the concrete complement against the negated source formula.
func (r *FormulaCheckedTable) Complement() (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	table, err := r.table.Complement()
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	source, err := r.manager.RefGet(r.formula)
	if err != nil {
		return nil, err
	}
	negated, err := r.manager.MkNot(source)
	if err != nil {
		return nil, err
	}
	ref, err := r.manager.NewRef(negated)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&ref)) }()
	actual, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = r.verifyGrounded(negated, actual, false); err != nil {
		return nil, fmt.Errorf("complement: %w", err)
	}
	return result, nil
}

// ToFormula returns an owned reference to the checked relation formula.
func (r *FormulaCheckedTable) ToFormula() (ast.ASTRef, error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	formula, err := r.manager.RefGet(r.formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return r.manager.NewRef(formula)
}
