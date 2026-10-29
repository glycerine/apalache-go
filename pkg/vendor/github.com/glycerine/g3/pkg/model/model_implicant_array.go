package model

// Source: src/model/model_implicant.cpp:468-616,875-917.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (i *ModelImplicant) isArray(h ast.ASTHandle) (bool, error) {
	s, err := i.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	family, err := i.manager.GetFamilyID(s)
	if err != nil {
		return false, err
	}
	arrayFamily, err := i.manager.ArrayFamilyID()
	return family == arrayFamily, err
}

func (i *ModelImplicant) extractArray(h ast.ASTHandle) (frame *arrayExpansion, ok bool, err error) {
	f := &arrayExpansion{active: true}
	f.elseCase, err = i.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return f, false, err
	}
	ok, _, err = i.evaluator.cfg.extractArray(h, f)
	return f, ok, err
}

func (i *ModelImplicant) evaluateRef(h ast.ASTHandle) (ast.ASTRef, error) {
	return i.evaluator.EvaluateValue(h)
}

func (i *ModelImplicant) valuesDiffer(left, right ast.ASTHandle) (bool, error) {
	leftValue, err := i.manager.IsValue(left)
	if err != nil {
		return false, err
	}
	rightValue, err := i.manager.IsValue(right)
	if err != nil {
		return false, err
	}
	return leftValue && rightValue && left != right, nil
}

func (i *ModelImplicant) evalArrayEq(h, left, right ast.ASTHandle) (err error) {
	leftRef, err := i.evaluateRef(left)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, i.manager.DelRef(&leftRef)) }()
	rightRef, err := i.evaluateRef(right)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, i.manager.DelRef(&rightRef)) }()
	leftValue, err := i.manager.RefGet(leftRef)
	if err != nil {
		return err
	}
	rightValue, err := i.manager.RefGet(rightRef)
	if err != nil {
		return err
	}
	if leftValue == rightValue {
		return i.setState(h, implicantTrue)
	}
	arraySort, err := i.manager.GetSort(left)
	if err != nil {
		return err
	}
	rangeSort, err := i.manager.ArrayRange(arraySort)
	if err != nil {
		return err
	}
	rangeInfinite, err := i.manager.SortIsInfinite(rangeSort)
	if err != nil {
		return err
	}
	rangeVeryBig, err := i.manager.SortIsVeryBig(rangeSort)
	if err != nil {
		return err
	}
	arrayInfinite, err := i.manager.SortIsInfinite(arraySort)
	if err != nil {
		return err
	}
	arrayVeryBig, err := i.manager.SortIsVeryBig(arraySort)
	if err != nil {
		return err
	}
	if !rangeInfinite && !rangeVeryBig && !arrayInfinite && !arrayVeryBig {
		return i.setState(h, implicantX)
	}
	first, firstOK, err := i.extractArray(leftValue)
	if first != nil {
		defer func() { err = errors.Join(err, first.close(i.manager)) }()
	}
	if err != nil {
		return err
	}
	if !firstOK {
		return i.setState(h, implicantX)
	}
	second, secondOK, err := i.extractArray(rightValue)
	if second != nil {
		defer func() { err = errors.Join(err, second.close(i.manager)) }()
	}
	if err != nil {
		return err
	}
	if !secondOK {
		return i.setState(h, implicantX)
	}
	else1, err := i.manager.RefGet(first.elseCase)
	if err != nil {
		return err
	}
	else2, err := i.manager.RefGet(second.elseCase)
	if err != nil {
		return err
	}
	if else1 != else2 {
		different, err := i.valuesDiffer(else1, else2)
		if err != nil {
			return err
		}
		if different {
			return i.setState(h, implicantFalse)
		}
		nested, err := i.isArray(else1)
		if err != nil {
			return err
		}
		if nested {
			return i.evalArrayEq(h, else1, else2)
		}
		return i.setState(h, implicantX)
	}
	rows := make([][]ast.ASTHandle, 0, len(first.stores)+len(second.stores))
	rows = append(rows, first.stores...)
	rows = append(rows, second.stores...)
	arrayFamily, err := i.manager.ArrayFamilyID()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) < 2 {
			return ast.ErrASTDomain
		}
		args1 := make([]ast.ASTHandle, 1, len(row))
		args1[0] = leftValue
		args1 = append(args1, row[:len(row)-1]...)
		args2 := make([]ast.ASTHandle, 1, len(row))
		args2[0] = rightValue
		args2 = append(args2, row[:len(row)-1]...)
		select1, err := i.manager.MkArrayAppArguments(arrayFamily, ast.ASTArrayOpSelect, ast.SliceArguments(args1))
		if err != nil {
			return err
		}
		select2, err := i.manager.MkArrayAppArguments(arrayFamily, ast.ASTArrayOpSelect, ast.SliceArguments(args2))
		if err != nil {
			return err
		}
		value1Ref, err := i.evaluateRef(select1)
		if err != nil {
			return err
		}
		value2Ref, err := i.evaluateRef(select2)
		if err != nil {
			_ = i.manager.DelRef(&value1Ref)
			return err
		}
		value1, firstErr := i.manager.RefGet(value1Ref)
		value2, secondErr := i.manager.RefGet(value2Ref)
		if err = errors.Join(firstErr, secondErr); err != nil {
			return errors.Join(err, i.manager.DelRef(&value2Ref), i.manager.DelRef(&value1Ref))
		}
		cleanupValues := func() error {
			return errors.Join(i.manager.DelRef(&value2Ref), i.manager.DelRef(&value1Ref))
		}
		if value1 == value2 {
			if err = cleanupValues(); err != nil {
				return err
			}
			continue
		}
		different, err := i.valuesDiffer(value1, value2)
		if err != nil {
			return errors.Join(err, cleanupValues())
		}
		if different {
			return errors.Join(i.setState(h, implicantFalse), cleanupValues())
		}
		nested, err := i.isArray(value1)
		if err != nil {
			return errors.Join(err, cleanupValues())
		}
		if nested {
			recursive := i.evalArrayEq(h, value1, value2)
			if err = errors.Join(recursive, cleanupValues()); err != nil {
				return err
			}
			if i.isTrue(h) {
				// The source recursively reuses e and then continues. Clear its
				// provisional true mark until every selected index is checked.
				delete(i.states, h)
				continue
			}
			return nil
		}
		if err = cleanupValues(); err != nil {
			return err
		}
		return i.setState(h, implicantX)
	}
	return i.setState(h, implicantTrue)
}
