package api

// Source: src/api/api_datatype.cpp mk_datatype_decl.

import "github.com/glycerine/g3/pkg/ast"

func (o *APIDatatypeConstructorObject) apiDefinition(owner *smtContextState, sortCount uint32) (ast.DatatypeConstructorDefinition, error) {
	if err := o.open(); err != nil {
		return ast.DatatypeConstructorDefinition{}, err
	}
	if o.owner != owner {
		return ast.DatatypeConstructorDefinition{}, ErrSMTContextOwner
	}
	for _, accessor := range o.definition.Accessors {
		if accessor.Recursive && accessor.RecursiveIndex >= sortCount {
			return ast.DatatypeConstructorDefinition{}, ast.ErrASTDomain
		}
	}
	result := o.definition
	result.Accessors = append([]ast.DatatypeAccessorDefinition(nil), o.definition.Accessors...)
	return result, nil
}

func (o *APIDatatypeConstructorObject) setConstructor(value ast.ASTHandle) error {
	if err := o.open(); err != nil {
		return err
	}
	manager, err := o.owner.command.Manager()
	if err != nil {
		return err
	}
	ref, err := manager.NewRef(value)
	if err != nil {
		return err
	}
	if o.constructorRef != (ast.ASTRef{}) {
		if err = manager.DelRef(&o.constructorRef); err != nil {
			_ = manager.DelRef(&ref)
			return err
		}
	}
	o.constructorRef = ref
	o.constructor = value
	return nil
}
