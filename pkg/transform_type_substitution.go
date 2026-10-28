package apalache

import "fmt"

// TypeSubstitutor applies a type-variable substitution to every typed tag in
// an expression, declaration, or module while preserving IR structure.
type TypeSubstitutor struct {
	tracker      TransformationTracker
	substitution *Substitution
}

func NewTypeSubstitutor(tracker TransformationTracker, substitution *Substitution) *TypeSubstitutor {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	if substitution == nil {
		substitution = EmptySubstitution()
	}
	return &TypeSubstitutor{tracker: tracker, substitution: substitution}
}

func (substitutor *TypeSubstitutor) tag(tag TypeTag) (TypeTag, error) {
	typ, ok := tag.Type()
	if !ok {
		return tag, nil
	}
	replaced, err := substitutor.substitution.ApplyRecursive(typ)
	if err != nil {
		return Untyped(), err
	}
	return Typed(replaced), nil
}

func (substitutor *TypeSubstitutor) Expression(expression Expr) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot substitute types in a nil expression")
	}
	tag, err := substitutor.tag(expression.Tag())
	if err != nil {
		return nil, err
	}
	var result Expr
	switch typed := expression.(type) {
	case *nullExpr:
		return expression, nil
	case *ValueExpr:
		if typed.Tag().Equal(tag) {
			result = typed
		} else {
			result = typed.WithTag(tag)
		}
	case *NameExpr:
		if typed.Tag().Equal(tag) {
			result = typed
		} else {
			result = typed.WithTag(tag)
		}
	case *OperExpr:
		arguments := typed.Arguments()
		changed := false
		for index, argument := range arguments {
			arguments[index], err = substitutor.Expression(argument)
			if err != nil {
				return nil, err
			}
			changed = changed || arguments[index].ID() != argument.ID()
		}
		result = typed
		if changed {
			result = substitutor.tracker.Hold(typed, MustOperExpr(typed.Operator(), typed.Tag(), arguments...))
		}
		if !result.Tag().Equal(tag) {
			result = result.WithTag(tag)
		}
	case *LetInExpr:
		body, bodyErr := substitutor.Expression(typed.Body())
		if bodyErr != nil {
			return nil, bodyErr
		}
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			declarations[index], err = substitutor.OperatorDeclaration(declaration)
			if err != nil {
				return nil, err
			}
		}
		result = NewLetInExpr(body, tag, declarations...)
	default:
		return nil, fmt.Errorf("unsupported expression %T", expression)
	}
	return substitutor.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
}

func (substitutor *TypeSubstitutor) OperatorDeclaration(declaration *OperDecl) (*OperDecl, error) {
	if declaration == nil {
		return nil, fmt.Errorf("cannot substitute types in a nil declaration")
	}
	tag, err := substitutor.tag(declaration.Tag())
	if err != nil {
		return nil, err
	}
	body, err := substitutor.Expression(declaration.Body())
	if err != nil {
		return nil, err
	}
	result := NewOperDecl(declaration.Name(), declaration.FormalParams(), body, tag)
	result.SetRecursive(declaration.IsRecursive())
	return substitutor.tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return result })(declaration), nil
}

func (substitutor *TypeSubstitutor) Declaration(declaration Decl) (Decl, error) {
	if declaration == nil {
		return nil, fmt.Errorf("cannot substitute types in a nil declaration")
	}
	if operator, ok := declaration.(*OperDecl); ok {
		return substitutor.OperatorDeclaration(operator)
	}
	tag, err := substitutor.tag(declaration.Tag())
	if err != nil {
		return nil, err
	}
	var result Decl
	switch typed := declaration.(type) {
	case *ConstDecl:
		result = NewConstDecl(typed.Name(), tag)
	case *VarDecl:
		result = NewVarDecl(typed.Name(), tag)
	case *AssumeDecl:
		body, bodyErr := substitutor.Expression(typed.Body())
		if bodyErr != nil {
			return nil, bodyErr
		}
		if name, named := typed.DefinedName(); named {
			result = NewNamedAssumeDecl(name, body, tag)
		} else {
			result = NewUnnamedAssumeDecl(body, tag)
		}
	case *TheoremDecl:
		body, bodyErr := substitutor.Expression(typed.Body())
		if bodyErr != nil {
			return nil, bodyErr
		}
		result = NewTheoremDecl(typed.Name(), body, tag)
	default:
		return nil, fmt.Errorf("unsupported declaration %T", declaration)
	}
	return substitutor.tracker.TrackDeclaration(func(Decl) Decl { return result })(declaration), nil
}

func (substitutor *TypeSubstitutor) Module(module *Module) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot substitute types in a nil module")
	}
	declarations := module.Declarations()
	for index, declaration := range declarations {
		result, err := substitutor.Declaration(declaration)
		if err != nil {
			return nil, err
		}
		declarations[index] = result
	}
	result := NewModule(module.Name(), declarations)
	result.copyMetadataFrom(module)
	return result, nil
}
