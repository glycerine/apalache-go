package apalache

import (
	"errors"
	"fmt"
	"strings"
)

const OverridePrefix = "OVERRIDE_"

type OverrideError struct {
	Message string
	Body    Expr
}

func (err *OverrideError) Error() string { return err.Message }

func RewriteOverrides(module *Module, tracker TransformationTracker) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot rewrite overrides in a nil module")
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	overrides := make(map[string]*OperDecl)
	for _, declaration := range module.OperDeclarations() {
		if strings.HasPrefix(declaration.Name(), OverridePrefix) {
			overrides[strings.TrimPrefix(declaration.Name(), OverridePrefix)] = declaration
		}
	}
	replacedConstants := make(map[string]TypeTag)
	transformed := make([]Decl, 0, len(module.Declarations()))
	for _, declaration := range module.Declarations() {
		if strings.HasPrefix(declaration.Name(), OverridePrefix) {
			continue
		}
		override, found := overrides[declaration.Name()]
		if !found {
			transformed = append(transformed, declaration)
			continue
		}
		switch typed := declaration.(type) {
		case *ConstDecl:
			if len(override.FormalParams()) != 0 {
				return nil, &OverrideError{Message: fmt.Sprintf("replacing constant %s with an operator body that has %d parameters", typed.Name(), len(override.FormalParams())), Body: override.Body()}
			}
			if !typed.Tag().Equal(override.Body().Tag()) {
				return nil, &OverrideError{Message: fmt.Sprintf("the types of %s (%s) and %s (%s) do not match", typed.Name(), typed.Tag(), override.Name(), override.Body().Tag()), Body: override.Body()}
			}
			constantType, ok := typed.Tag().Type()
			if !ok {
				return nil, &OverrideError{Message: fmt.Sprintf("constant %s is untyped", typed.Name()), Body: override.Body()}
			}
			transformed = append(transformed, NewOperDecl(typed.Name(), nil, override.Body(), Typed(mustOperator(nil, constantType))))
			replacedConstants[typed.Name()] = typed.Tag()
		case *OperDecl:
			if len(override.FormalParams()) != len(typed.FormalParams()) {
				return nil, &OverrideError{Message: fmt.Sprintf("replacing operator %s (%d) with %s (%d)", typed.Name(), len(typed.FormalParams()), override.Name(), len(override.FormalParams())), Body: override.Body()}
			}
			copy := NewOperDecl(typed.Name(), override.FormalParams(), override.Body(), override.Tag())
			copy.SetRecursive(override.IsRecursive())
			transformed = append(transformed, copy)
		case *VarDecl:
			return nil, &OverrideError{Message: fmt.Sprintf("trying to replace variable %s with %s%s; use INSTANCE WITH", typed.Name(), OverridePrefix, typed.Name()), Body: NullExpr()}
		default:
			transformed = append(transformed, declaration)
		}
	}
	result, err := SortDeclarations(NewModule(module.Name(), transformed))
	if err != nil {
		var cycle *CyclicDependencyError
		if errors.As(err, &cycle) {
			return nil, &ConfigurationError{Message: cycle.Error()}
		}
		return nil, err
	}
	for name, tag := range replacedConstants {
		constantType, _ := tag.Type()
		operatorType := mustOperator(nil, constantType)
		transform := ReplaceMatching(tracker, func(expression Expr) bool {
			named, ok := expression.(*NameExpr)
			return ok && named.Name() == name && named.Tag().Equal(tag)
		}, func() Expr { return MustOperExpr(Op("OPER_APP"), tag, NewNameExpr(name, Typed(operatorType))) })
		result = transformAllModuleExpressions(result, transform, tracker)
	}
	// Module properties are Go's pass-order contract and have no Scala IR
	// counterpart. Configuration still requires the input's TypeChecked marker
	// after applying overrides, so preserve these adaptation-level properties.
	result.properties = cloneMap(module.properties)
	return result, nil
}

func transformAllModuleExpressions(module *Module, transform ExprTransform, tracker TransformationTracker) *Module {
	declarations := module.Declarations()
	for index, declaration := range declarations {
		var result Decl = declaration
		switch typed := declaration.(type) {
		case *OperDecl:
			copy := typed.copyWithTag(typed.Tag())
			copy.SetBody(transform(typed.Body()))
			result = copy
		case *AssumeDecl:
			body := transform(typed.Body())
			if name, named := typed.DefinedName(); named {
				result = NewNamedAssumeDecl(name, body, typed.Tag())
			} else {
				result = NewUnnamedAssumeDecl(body, typed.Tag())
			}
		case *TheoremDecl:
			result = NewTheoremDecl(typed.Name(), transform(typed.Body()), typed.Tag())
		}
		if result != declaration {
			result = tracker.TrackDeclaration(func(Decl) Decl { return result })(declaration)
		}
		declarations[index] = result
	}
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	return result
}
