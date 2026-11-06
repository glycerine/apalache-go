package rel

// Source: check_relation_plugin operations over karr_relation.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// KarrRelationFormulaChecker verifies one concrete Karr relation against the
// symbolic postconditions used by check_relation_plugin.
type KarrRelationFormulaChecker struct {
	manager *ast.ASTManager
	config  smtcore.SMTSolverConfig
}

func NewKarrRelationFormulaChecker(manager *ast.ASTManager, config smtcore.SMTSolverConfig) (*KarrRelationFormulaChecker, error) {
	if manager == nil {
		return nil, ErrRelationVerification
	}
	return &KarrRelationFormulaChecker{manager: manager, config: config}, nil
}

func (c *KarrRelationFormulaChecker) sorts(dimension uint32) ([]ast.ASTHandle, error) {
	if c == nil || c.manager == nil {
		return nil, ErrRelationVerification
	}
	family, err := c.manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	integer, err := c.manager.MkFamilySort(family, 1, nil)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, dimension)
	for i := range sorts {
		sorts[i] = integer
	}
	return sorts, nil
}

func (c *KarrRelationFormulaChecker) formula(relation *KarrRelation) (ast.ASTRef, []ast.ASTHandle, error) {
	if c == nil || relation == nil {
		return ast.ASTRef{}, nil, ErrRelationVerification
	}
	sorts, err := c.sorts(relation.dimension)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	formula, err := relation.ToFormula(c.manager, sorts)
	return formula, sorts, err
}

func (c *KarrRelationFormulaChecker) verify(expected ast.ASTHandle, actual *KarrRelation) error {
	if c == nil || actual == nil {
		return ErrRelationVerification
	}
	sorts, err := c.sorts(actual.dimension)
	if err != nil {
		return err
	}
	return VerifyKarrRelationExpected(c.manager, sorts, c.config, expected, actual)
}

func (c *KarrRelationFormulaChecker) VerifyConstant(actual *KarrRelation, full bool) error {
	if c == nil || actual == nil {
		return ErrRelationVerification
	}
	sorts, err := c.sorts(actual.dimension)
	if err != nil {
		return err
	}
	return VerifyKarrRelationConstant(c.manager, sorts, c.config, actual, full)
}

func (c *KarrRelationFormulaChecker) VerifyFactAdd(values []ast.ASTHandle, actual *KarrRelation) (resultErr error) {
	if c == nil || actual == nil || len(values) != int(actual.dimension) {
		return ErrRelationVerification
	}
	sorts, err := c.sorts(actual.dimension)
	if err != nil {
		return err
	}
	before, err := c.manager.MkFalse()
	if err != nil {
		return err
	}
	beforeRef, err := c.manager.NewRef(before)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&beforeRef)) }()
	return VerifyKarrRelationFactAdd(c.manager, sorts, c.config, before, values, actual)
}

func (c *KarrRelationFormulaChecker) verifyProject(source *KarrRelation, removed []uint32, actual *KarrRelation) (resultErr error) {
	formula, sorts, err := c.formula(source)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&formula)) }()
	sourceHandle, err := c.manager.RefGet(formula)
	if err != nil {
		return err
	}
	removedColumns := make([]int, len(removed))
	for i, column := range removed {
		removedColumns[i] = int(column)
	}
	checker := relationFormulaChecker{manager: c.manager, sorts: sorts, config: c.config}
	expected, err := checker.projectFormulaForSorts(sourceHandle, sorts, removedColumns)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&expected)) }()
	expectedHandle, err := c.manager.RefGet(expected)
	if err != nil {
		return err
	}
	return c.verify(expectedHandle, actual)
}

func (c *KarrRelationFormulaChecker) verifyRename(source *KarrRelation, cycle []uint32, actual *KarrRelation) (resultErr error) {
	formula, sorts, err := c.formula(source)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&formula)) }()
	checker := relationFormulaChecker{manager: c.manager, sorts: sorts, config: c.config}
	expected, err := checker.renameFormulaRef(formula, sorts, sorts, cycle)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&expected)) }()
	expectedHandle, err := c.manager.RefGet(expected)
	if err != nil {
		return err
	}
	return c.verify(expectedHandle, actual)
}

func (c *KarrRelationFormulaChecker) verifyJoin(left, right *KarrRelation, leftColumns, rightColumns []uint32, actual *KarrRelation) (resultErr error) {
	leftFormula, leftSorts, err := c.formula(left)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&leftFormula)) }()
	rightFormula, rightSorts, err := c.formula(right)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&rightFormula)) }()
	checker := relationFormulaChecker{manager: c.manager, config: c.config}
	expected, err := checker.joinFormulaRefs(leftFormula, rightFormula, leftSorts, rightSorts, leftColumns, rightColumns)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&expected)) }()
	expectedHandle, err := c.manager.RefGet(expected)
	if err != nil {
		return err
	}
	return c.verify(expectedHandle, actual)
}

func (c *KarrRelationFormulaChecker) verifyUnion(before, source, actual *KarrRelation) (resultErr error) {
	beforeFormula, sorts, err := c.formula(before)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&beforeFormula)) }()
	sourceFormula, _, err := c.formula(source)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&sourceFormula)) }()
	beforeHandle, err := c.manager.RefGet(beforeFormula)
	if err != nil {
		return err
	}
	sourceHandle, err := c.manager.RefGet(sourceFormula)
	if err != nil {
		return err
	}
	expected, err := c.manager.MkOrArgs([]ast.ASTHandle{beforeHandle, sourceHandle})
	if err != nil {
		return err
	}
	expectedRef, err := c.manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&expectedRef)) }()
	return VerifyKarrRelationExpected(c.manager, sorts, c.config, expected, actual)
}

func (c *KarrRelationFormulaChecker) verifyFilter(before *KarrRelation, condition ast.ASTHandle, actual *KarrRelation) (resultErr error) {
	if c == nil || condition.IsNull() || before == nil {
		return ErrRelationVerification
	}
	formula, sorts, err := c.formula(before)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&formula)) }()
	beforeHandle, err := c.manager.RefGet(formula)
	if err != nil {
		return err
	}
	expected, err := c.manager.MkAndArgs([]ast.ASTHandle{beforeHandle, condition})
	if err != nil {
		return err
	}
	expectedRef, err := c.manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&expectedRef)) }()
	return VerifyKarrRelationExpected(c.manager, sorts, c.config, expected, actual)
}

func (c *KarrRelationFormulaChecker) equalityCondition(dimension, column uint32, value ast.ASTHandle) (ast.ASTRef, error) {
	if c == nil || column >= dimension || value.IsNull() {
		return ast.ASTRef{}, ErrRelationVerification
	}
	sorts, err := c.sorts(dimension)
	if err != nil {
		return ast.ASTRef{}, err
	}
	variable, err := c.manager.MkVar(column, sorts[column])
	if err != nil {
		return ast.ASTRef{}, err
	}
	equality, err := c.manager.MkEq(variable, value)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(equality)
}

func (c *KarrRelationFormulaChecker) VerifyFilterEqual(before *KarrRelation, column uint32, value ast.ASTHandle, actual *KarrRelation) error {
	if before == nil || actual == nil {
		return ErrRelationVerification
	}
	condition, err := c.equalityCondition(before.dimension, column, value)
	if err != nil {
		return err
	}
	defer c.manager.DelRef(&condition)
	handle, err := c.manager.RefGet(condition)
	if err != nil {
		return err
	}
	return c.verifyFilter(before, handle, actual)
}

func (c *KarrRelationFormulaChecker) VerifyFilter(before *KarrRelation, condition ast.ASTHandle, actual *KarrRelation) error {
	return c.verifyFilter(before, condition, actual)
}

func (c *KarrRelationFormulaChecker) identicalCondition(dimension uint32, columns []uint32) (ast.ASTRef, error) {
	if c == nil || len(columns) == 0 {
		return ast.ASTRef{}, ErrRelationVerification
	}
	sorts, err := c.sorts(dimension)
	if err != nil {
		return ast.ASTRef{}, err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			_ = c.manager.DelRef(&refs[i])
		}
	}()
	var equalities []ast.ASTHandle
	for _, column := range columns[1:] {
		if columns[0] >= dimension || column >= dimension {
			return ast.ASTRef{}, ErrRelationVerification
		}
		left, err := c.manager.MkVar(columns[0], sorts[columns[0]])
		if err != nil {
			return ast.ASTRef{}, err
		}
		right, err := c.manager.MkVar(column, sorts[column])
		if err != nil {
			return ast.ASTRef{}, err
		}
		equality, err := c.manager.MkEq(left, right)
		if err != nil {
			return ast.ASTRef{}, err
		}
		ref, err := c.manager.NewRef(equality)
		if err != nil {
			return ast.ASTRef{}, err
		}
		refs = append(refs, ref)
		equalities = append(equalities, equality)
	}
	condition, err := c.manager.MkAndArray(equalities)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(condition)
}

func (c *KarrRelationFormulaChecker) linearCondition(dimension uint32, row []big.Rat, constant *big.Rat, equal bool) (ast.ASTRef, error) {
	if c == nil || len(row) != int(dimension) {
		return ast.ASTRef{}, ErrRelationVerification
	}
	sorts, err := c.sorts(dimension)
	if err != nil {
		return ast.ASTRef{}, err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			_ = c.manager.DelRef(&refs[i])
		}
	}()
	hold := func(h ast.ASTHandle, e error) (ast.ASTHandle, error) {
		if e != nil {
			return ast.ASTHandle{}, e
		}
		ref, e := c.manager.NewRef(h)
		if e == nil {
			refs = append(refs, ref)
		}
		return h, e
	}
	zero, err := hold(c.manager.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return ast.ASTRef{}, err
	}
	terms := make([]ast.ASTHandle, 0, len(row)+1)
	for i := range row {
		if row[i].Sign() == 0 {
			continue
		}
		variable, e := hold(c.manager.MkVar(uint32(i), sorts[i]))
		if e != nil {
			return ast.ASTRef{}, e
		}
		if row[i].Cmp(big.NewRat(1, 1)) == 0 {
			terms = append(terms, variable)
			continue
		}
		coefficient, e := hold(c.manager.ArithmeticNumeralRat(&row[i], true))
		if e != nil {
			return ast.ASTRef{}, e
		}
		term, e := hold(c.manager.MkArithmeticMul(coefficient, variable))
		if e != nil {
			return ast.ASTRef{}, e
		}
		terms = append(terms, term)
	}
	if constant != nil && constant.Sign() != 0 {
		term, e := hold(c.manager.ArithmeticNumeralRat(constant, true))
		if e != nil {
			return ast.ASTRef{}, e
		}
		terms = append(terms, term)
	}
	lhs := zero
	if len(terms) > 0 {
		lhs = terms[0]
		for _, term := range terms[1:] {
			lhs, err = hold(c.manager.MkArithmeticAdd(lhs, term))
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
	}
	var condition ast.ASTHandle
	if equal {
		condition, err = c.manager.MkEq(lhs, zero)
	} else {
		condition, err = c.manager.MkArithmeticGE(lhs, zero)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(condition)
}
