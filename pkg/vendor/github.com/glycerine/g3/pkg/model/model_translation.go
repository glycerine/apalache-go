package model

// Source: src/model/func_interp.cpp:328-341; src/model/model.cpp:152-182.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Translate copies a function graph through an ASTTranslation. The returned
// graph owns all translated entries and its fallback.
func (f *FunctionInterpretation) Translate(translation *ast.ASTTranslation) (result *FunctionInterpretation, err error) {
	if err = f.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != f.manager || translation.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err = NewFunctionInterpretation(translation.To(), f.arity)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	args := make([]ast.ASTHandle, f.arity)
	for _, entry := range f.entries {
		for n := uint32(0); n < f.arity; n++ {
			arg, e := entry.Argument(n)
			if e != nil {
				return nil, e
			}
			args[n], e = translation.Translate(arg)
			if e != nil {
				return nil, e
			}
		}
		value, e := entry.Result()
		if e != nil {
			return nil, e
		}
		value, e = translation.Translate(value)
		if e != nil {
			return nil, e
		}
		if e = result.InsertNewEntry(args, value); e != nil {
			return nil, e
		}
	}
	fallback, err := translation.Translate(f.fallback)
	if err != nil {
		return nil, err
	}
	if err = result.SetElse(fallback); err != nil {
		return nil, err
	}
	return result, nil
}

// Translate copies constants, function graphs, and universes in their source
// dictionary order through a shared AST translation cache.
func (md *Model) Translate(translation *ast.ASTTranslation) (result *Model, err error) {
	if err = md.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != md.CoreField.manager || translation.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err = NewModel(translation.To())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for declaration, entry := range md.CoreField.Constants.All() {
		translatedDeclaration, e := translation.Translate(declaration)
		if e != nil {
			return nil, e
		}
		translatedValue, e := translation.Translate(entry.value)
		if e != nil {
			return nil, e
		}
		if e = result.Core().RegisterConstant(translatedDeclaration, translatedValue); e != nil {
			return nil, e
		}
	}
	for declaration, entry := range md.CoreField.Functions.All() {
		translatedDeclaration, e := translation.Translate(declaration)
		if e != nil {
			return nil, e
		}
		translatedFunction, e := entry.function.Translate(translation)
		if e != nil {
			return nil, e
		}
		if e = result.Core().RegisterFunction(translatedDeclaration, translatedFunction); e != nil {
			return nil, errors.Join(e, translatedFunction.Close())
		}
	}
	for sort, universe := range md.Universes.All() {
		translatedSort, e := translation.Translate(sort)
		if e != nil {
			return nil, e
		}
		values := make([]ast.ASTHandle, len(universe.values))
		for n, value := range universe.values {
			values[n], e = translation.Translate(value)
			if e != nil {
				return nil, e
			}
		}
		if e = result.RegisterUniverse(translatedSort, values); e != nil {
			return nil, e
		}
	}
	return result, nil
}
