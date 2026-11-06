package rel

// Source: src/muz/rel/karr_relation.cpp karr_relation::to_formula.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

// ToFormula returns the integer linear formula represented by the Karr
// inequalities. The returned AST reference is owned by the caller.
func (r *KarrRelation) ToFormula(manager *ast.ASTManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if r == nil || manager == nil || len(sorts) != int(r.dimension) {
		return result, ErrKarrDimension
	}
	if r.empty {
		value, err := manager.MkFalse()
		if err != nil {
			return result, err
		}
		return manager.NewRef(value)
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return result, err
	}
	var roots []ast.ASTRef
	owned := false
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&roots[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := manager.NewRef(value)
		if err == nil {
			roots = append(roots, ref)
		}
		return value, err
	}
	variables := make([]ast.ASTHandle, len(sorts))
	for i, sort := range sorts {
		sortFamily, e := manager.GetFamilyID(sort)
		if e != nil {
			return result, e
		}
		kind, e := manager.GetDeclKind(sort)
		if e != nil {
			return result, e
		}
		if sortFamily != family || kind != 1 {
			return result, ErrKarrDimension
		}
		variables[i], e = hold(manager.MkVar(uint32(i), sort))
		if e != nil {
			return result, e
		}
	}
	matrix, err := r.Inequalities()
	if err != nil {
		return result, err
	}
	zero, err := hold(manager.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return result, err
	}
	conjuncts := make([]ast.ASTHandle, 0, len(matrix.A))
	for rowIndex, row := range matrix.A {
		if len(row) != len(variables) {
			return result, ErrKarrDimension
		}
		terms := make([]ast.ASTHandle, 0, len(row)+1)
		for column := range row {
			coefficient := &row[column]
			if coefficient.Sign() == 0 {
				continue
			}
			if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
				terms = append(terms, variables[column])
				continue
			}
			numeral, e := hold(manager.ArithmeticNumeralRat(coefficient, true))
			if e != nil {
				return result, e
			}
			product, e := hold(manager.MkArithmeticMul(numeral, variables[column]))
			if e != nil {
				return result, e
			}
			terms = append(terms, product)
		}
		if matrix.B[rowIndex].Sign() != 0 {
			constant, e := hold(manager.ArithmeticNumeralRat(&matrix.B[rowIndex], true))
			if e != nil {
				return result, e
			}
			terms = append(terms, constant)
		}
		var lhs ast.ASTHandle
		if len(terms) == 0 {
			lhs = zero
		} else {
			lhs = terms[0]
			for _, term := range terms[1:] {
				lhs, err = hold(manager.MkArithmeticAdd(lhs, term))
				if err != nil {
					return result, err
				}
			}
		}
		var formula ast.ASTHandle
		if matrix.Eq[rowIndex] {
			formula, err = manager.MkEq(lhs, zero)
		} else {
			formula, err = manager.MkArithmeticGE(lhs, zero)
		}
		formula, err = hold(formula, err)
		if err != nil {
			return result, err
		}
		conjuncts = append(conjuncts, formula)
	}
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	owned = true
	if err = rewriter.SimplifyBooleanGateDefault(manager, ast.ASTOpAnd, conjuncts, result); err != nil {
		return result, err
	}
	return result, nil
}

func VerifyKarrRelationExpected(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, expected ast.ASTHandle, actual *KarrRelation) (resultErr error) {
	if manager == nil || expected.IsNull() || actual == nil {
		return ErrRelationVerification
	}
	formula, err := actual.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&formula)) }()
	actualFormula, err := manager.RefGet(formula)
	if err != nil {
		return err
	}
	return verifyRelationFormulaPair(manager, sorts, config, expected, actualFormula, false)
}

func VerifyKarrRelationConstant(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, actual *KarrRelation, full bool) error {
	if manager == nil || actual == nil {
		return ErrRelationVerification
	}
	var expected ast.ASTHandle
	var err error
	if full {
		expected, err = manager.MkTrue()
	} else {
		expected, err = manager.MkFalse()
	}
	if err != nil {
		return err
	}
	ref, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer manager.DelRef(&ref)
	return VerifyKarrRelationExpected(manager, sorts, config, expected, actual)
}

func VerifyKarrRelationFactAdd(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before ast.ASTHandle, values []ast.ASTHandle, after *KarrRelation) (resultErr error) {
	if manager == nil || before.IsNull() || len(sorts) != len(values) || after == nil {
		return ErrRelationVerification
	}
	checker := relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), config: config}
	fact, err := checker.factFormula(values)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&fact)) }()
	factFormula, err := manager.RefGet(fact)
	if err != nil {
		return err
	}
	expected, err := manager.MkOrArgs([]ast.ASTHandle{before, factFormula})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedRef)) }()
	return VerifyKarrRelationExpected(manager, sorts, config, expected, after)
}
