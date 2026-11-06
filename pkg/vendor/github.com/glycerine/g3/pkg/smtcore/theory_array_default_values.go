package smtcore

// Source: src/smt/theory_array_full.cpp default constructor value expressions.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTArrayTheory) defaultConstructorValue(array ast.ASTHandle, op int32) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var value ast.ASTHandle
	switch op {
	case ast.ASTArrayOpConst:
		value, err = state.manager.GetArg(array, 0)
	case ast.ASTArrayOpMap:
		declaration, declErr := state.manager.GetDecl(array)
		if declErr != nil {
			return ast.ASTHandle{}, declErr
		}
		function, functionErr := state.manager.ArrayMapFunction(declaration)
		if functionErr != nil {
			return ast.ASTHandle{}, functionErr
		}
		count, countErr := state.manager.GetNumArgs(array)
		if countErr != nil {
			return ast.ASTHandle{}, countErr
		}
		arity, arityErr := state.manager.GetArity(function)
		if arityErr != nil {
			return ast.ASTHandle{}, arityErr
		}
		if count == 0 || count != arity {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		arguments := make([]ast.ASTHandle, 0, count)
		for index := uint32(0); index < count; index++ {
			input, argErr := state.manager.GetArg(array, index)
			if argErr != nil {
				return ast.ASTHandle{}, argErr
			}
			inputDefault, defaultErr := state.manager.MkFamilyApp(t.FamilyID(), ast.ASTArrayOpDefault, nil, []ast.ASTHandle{input})
			if defaultErr != nil {
				return ast.ASTHandle{}, defaultErr
			}
			arguments = append(arguments, inputDefault)
		}
		value, err = state.manager.MkApp(function, arguments)
	case ast.ASTArrayOpStore:
		count, countErr := state.manager.GetNumArgs(array)
		if countErr != nil || count < 3 {
			if countErr != nil {
				return ast.ASTHandle{}, countErr
			}
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		large, largeErr := t.arrayHasLargeDomain(array)
		if largeErr != nil {
			return ast.ASTHandle{}, largeErr
		}
		base, argErr := state.manager.GetArg(array, 0)
		if argErr != nil {
			return ast.ASTHandle{}, argErr
		}
		baseDefault, defaultErr := state.manager.MkFamilyApp(t.FamilyID(), ast.ASTArrayOpDefault, nil, []ast.ASTHandle{base})
		if defaultErr != nil {
			return ast.ASTHandle{}, defaultErr
		}
		if large {
			value = baseDefault
			break
		}
		conditions := make([]ast.ASTHandle, 0, count-2)
		for index := uint32(1); index+1 < count; index++ {
			location, locationErr := state.manager.GetArg(array, index)
			if locationErr != nil {
				return ast.ASTHandle{}, locationErr
			}
			sort, sortErr := state.manager.GetSort(location)
			if sortErr != nil {
				return ast.ASTHandle{}, sortErr
			}
			epsilon, epsilonErr := t.arrayEpsilon(sort)
			if epsilonErr != nil {
				return ast.ASTHandle{}, epsilonErr
			}
			equal, equalErr := state.manager.MkEq(epsilon, location)
			if equalErr != nil {
				return ast.ASTHandle{}, equalErr
			}
			conditions = append(conditions, equal)
		}
		condition, conditionErr := state.manager.MkAndArray(conditions)
		if conditionErr != nil {
			return ast.ASTHandle{}, conditionErr
		}
		storedValue, valueErr := state.manager.GetArg(array, count-1)
		if valueErr != nil {
			return ast.ASTHandle{}, valueErr
		}
		value, err = state.manager.MkIte(condition, storedValue, baseDefault)
	case ast.ASTArrayOpAsArray:
		function, functionErr := state.manager.AsArrayFunction(array)
		if functionErr != nil {
			return ast.ASTHandle{}, functionErr
		}
		arity, arityErr := state.manager.GetArity(function)
		if arityErr != nil {
			return ast.ASTHandle{}, arityErr
		}
		arguments := make([]ast.ASTHandle, 0, arity)
		for index := uint32(0); index < arity; index++ {
			sort, domainErr := state.manager.GetDomain(function, index)
			if domainErr != nil {
				return ast.ASTHandle{}, domainErr
			}
			epsilon, epsilonErr := t.arrayEpsilon(sort)
			if epsilonErr != nil {
				return ast.ASTHandle{}, epsilonErr
			}
			arguments = append(arguments, epsilon)
		}
		value, err = state.manager.MkApp(function, arguments)
	default:
		return ast.ASTHandle{}, nil
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return value, nil
}
