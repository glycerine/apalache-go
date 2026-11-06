package api

// Source: src/api/api_datatype.cpp:230-263,308-312.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// APIDatatypeConstructorObject owns the temporary sort references held by a
// native Z3_constructor. It may be reused only while its context is open.
type APIDatatypeConstructorObject struct {
	self           *APIDatatypeConstructorObject
	owner          *smtContextState
	definition     ast.DatatypeConstructorDefinition
	sortRefs       []ast.ASTRef
	constructorRef ast.ASTRef
	constructor    ast.ASTHandle
	closed         bool
}

func (c *SMTContext) NewDatatypeConstructorObject(definition APIDatatypeConstructor) (result *APIDatatypeConstructorObject, resultErr error) {
	converted, err := c.datatypeConstructorSymbols(definition)
	if err != nil {
		return nil, err
	}
	return c.NewDatatypeConstructorObjectWithSymbols(converted)
}

func (c *SMTContext) datatypeConstructorSymbols(definition APIDatatypeConstructor) (result ast.DatatypeConstructorDefinition, err error) {
	if result.Name, err = c.StringSymbol(definition.Name); err != nil {
		return result, err
	}
	if result.Recognizer, err = c.StringSymbol(definition.Recognizer); err != nil {
		return result, err
	}
	for _, field := range definition.Accessors {
		name, err := c.StringSymbol(field.Name)
		if err != nil {
			return result, err
		}
		result.Accessors = append(result.Accessors, ast.DatatypeAccessorDefinition{Name: name, Range: field.Sort, Recursive: field.Recursive, RecursiveIndex: field.RecursiveIndex})
	}
	return result, nil
}

// NewDatatypeConstructorObjectWithSymbols owns a snapshot of constructor
// metadata and concrete field sorts; its names borrow the context symbol owner.
func (c *SMTContext) NewDatatypeConstructorObjectWithSymbols(definition ast.DatatypeConstructorDefinition) (result *APIDatatypeConstructorObject, resultErr error) {
	manager, err := c.datatypeManager()
	if err != nil {
		return nil, err
	}
	if _, err = manager.Symbols.Hash(definition.Name); err != nil {
		return nil, err
	}
	if _, err = manager.Symbols.Hash(definition.Recognizer); err != nil {
		return nil, err
	}
	result = &APIDatatypeConstructorObject{owner: c.smtContextState}
	result.self = result
	result.definition.Name = definition.Name
	result.definition.Recognizer = definition.Recognizer
	result.definition.Accessors = append(result.definition.Accessors, definition.Accessors...)
	result.sortRefs = make([]ast.ASTRef, len(definition.Accessors))
	ownedResult := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, ownedResult.Close())
			result = nil
		}
	}()
	for i, accessor := range result.definition.Accessors {
		if _, err = manager.Symbols.Hash(accessor.Name); err != nil {
			return nil, err
		}
		if accessor.Recursive != accessor.Range.IsNull() {
			return nil, ast.ErrASTDomain
		}
		if accessor.Recursive {
			continue
		}
		kind, kindErr := manager.GetKind(accessor.Range)
		if kindErr != nil {
			return nil, kindErr
		}
		if kind != ast.ASTSort {
			return nil, ast.ErrASTDomain
		}
		result.sortRefs[i], err = manager.NewRef(accessor.Range)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (o *APIDatatypeConstructorObject) open() error {
	if o == nil || o.self != o || o.owner == nil || o.closed || o.owner.closed || o.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (o *APIDatatypeConstructorObject) Close() (result error) {
	if o == nil || o.closed {
		return nil
	}
	if o.self != o || o.owner == nil || o.owner.command == nil {
		return ErrSMTContextOwner
	}
	manager, err := o.owner.command.Manager()
	if err != nil {
		return err
	}
	if o.constructorRef != (ast.ASTRef{}) {
		result = errors.Join(result, manager.DelRef(&o.constructorRef))
	}
	for i := range o.sortRefs {
		if o.sortRefs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&o.sortRefs[i]))
		}
	}
	if result == nil {
		o.owner = nil
		o.definition = ast.DatatypeConstructorDefinition{}
		o.sortRefs = nil
		o.constructor = ast.ASTHandle{}
		o.closed = true
	}
	return result
}
