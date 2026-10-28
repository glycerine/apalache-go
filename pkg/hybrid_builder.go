package apalache

import "fmt"

// OperDeclInstruction is a delayed scope-safe operator declaration.
type OperDeclInstruction struct {
	run func(*scopeContext) (*OperDecl, error)
}

func (instruction OperDeclInstruction) Build() (*OperDecl, error) {
	if instruction.run == nil {
		return nil, fmt.Errorf("empty declaration instruction")
	}
	return instruction.run(newScopeContext())
}

func (builder *TypedBuilder) Unchecked(expr Expr) Expr { return expr }

func (builder *TypedBuilder) UncheckedDecl(declaration *OperDecl) *OperDecl { return declaration }

func (builder *TypedBuilder) Param(name string, typ Type) (TypedParameter, error) {
	return NewTypedParameter(name, typ)
}

func (builder *ScopedBuilder) Param(name string, typ Type) (TypedParameter, error) {
	return NewTypedParameter(name, typ)
}

func (builder *TypedBuilder) PrimeEq(left, right Expr) (*OperExpr, error) {
	primed, err := builder.Prime(left)
	if err != nil {
		return nil, err
	}
	return builder.Eql(primed, right)
}

func (builder *ScopedBuilder) PrimeEq(left, right ExprInstruction) ExprInstruction {
	return builder.Eql(builder.Prime(left), right)
}

func (builder *ScopedBuilder) UncheckedDecl(declaration *OperDecl) OperDeclInstruction {
	return OperDeclInstruction{run: func(*scopeContext) (*OperDecl, error) {
		if declaration == nil {
			return nil, fmt.Errorf("unchecked declaration is nil")
		}
		return declaration, nil
	}}
}

func (builder *ScopedBuilder) Declaration(name string, body ExprInstruction, parameters ...TypedParameter) OperDeclInstruction {
	return OperDeclInstruction{run: func(context *scopeContext) (*OperDecl, error) {
		for _, parameter := range parameters {
			if err := validateTypedParameter(parameter); err != nil {
				return nil, err
			}
		}
		bodyExpr, err := body.run(context)
		if err != nil {
			return nil, err
		}
		for _, parameter := range parameters {
			parameterExpr, runErr := builder.Name(parameter.Parameter.Name, parameter.Type).run(context)
			if runErr != nil {
				return nil, runErr
			}
			if _, runErr = markBound(context, parameterExpr); runErr != nil {
				return nil, runErr
			}
		}
		return builder.typed.Declaration(name, bodyExpr, parameters...)
	}}
}

func (builder *ScopedBuilder) DeclarationWithInferredParameterTypes(name string, body ExprInstruction, parameters ...OperatorParam) OperDeclInstruction {
	return OperDeclInstruction{run: func(context *scopeContext) (*OperDecl, error) {
		bodyExpr, err := body.run(context)
		if err != nil {
			return nil, err
		}
		typed := make([]TypedParameter, len(parameters))
		for index, parameter := range parameters {
			typ, exists := context.free[parameter.Name]
			if !exists {
				return nil, &ScopeError{Message: fmt.Sprintf("cannot infer type of parameter %s: it is not in scope", parameter.Name)}
			}
			typed[index] = TypedParameter{Parameter: parameter, Type: typ}
			if err = validateTypedParameter(typed[index]); err != nil {
				return nil, err
			}
			parameterExpr := builder.typed.Name(parameter.Name, typ)
			if _, err = markBound(context, parameterExpr); err != nil {
				return nil, err
			}
		}
		return builder.typed.Declaration(name, bodyExpr, typed...)
	}}
}

func (builder *ScopedBuilder) Lambda(uniqueName string, body ExprInstruction, parameters ...TypedParameter) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		bodyExpr, err := body.run(context)
		if err != nil {
			return nil, err
		}
		bodyType, err := ExpressionType(bodyExpr)
		if err != nil {
			return nil, err
		}
		parameterTypes := make([]Type, len(parameters))
		for index, parameter := range parameters {
			if err = validateTypedParameter(parameter); err != nil {
				return nil, err
			}
			parameterTypes[index] = parameter.Type
		}
		operatorType, err := NewOperatorType(parameterTypes, bodyType)
		if err != nil {
			return nil, err
		}
		return builder.LetIn(
			builder.Name(uniqueName, operatorType),
			builder.Declaration(uniqueName, body, parameters...),
		).run(context)
	}}
}

func (builder *ScopedBuilder) LetIn(body ExprInstruction, declarations ...OperDeclInstruction) ExprInstruction {
	if len(declarations) == 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("LetIn requires at least one declaration")
		}}
	}
	result := body
	for index := len(declarations) - 1; index >= 0; index-- {
		declaration := declarations[index]
		partial := result
		result = ExprInstruction{run: func(context *scopeContext) (Expr, error) {
			usedBefore := cloneNameSet(context.used)
			declarationExpr, err := declaration.run(context)
			if err != nil {
				return nil, err
			}
			usedAfterDeclaration := cloneNameSet(context.used)
			usedInDeclaration := differenceNameSets(usedAfterDeclaration, usedBefore)
			bodyExpr, err := partial.run(context)
			if err != nil {
				return nil, err
			}
			boundAfterBody := boundNames(context)
			boundInBody := differenceNameSets(boundAfterBody, usedAfterDeclaration)
			if _, used := usedInDeclaration[declarationExpr.Name()]; used {
				return nil, &ScopeError{Message: fmt.Sprintf("declaration name %s is shadowed in its body", declarationExpr.Name())}
			}
			if _, bound := boundInBody[declarationExpr.Name()]; bound {
				return nil, &ScopeError{Message: fmt.Sprintf("declaration name %s is shadowed in LET body", declarationExpr.Name())}
			}
			return builder.letInBuilt(context, builder.Unchecked(bodyExpr), declarationExpr)
		}}
	}
	return result
}

func (builder *ScopedBuilder) letInBuilt(context *scopeContext, body ExprInstruction, declaration *OperDecl) (Expr, error) {
	bodyExpr, err := body.run(context)
	if err != nil {
		return nil, err
	}
	typ, ok := declaration.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("LET declaration %s is untyped", declaration.Name())
	}
	nameExpr, err := builder.Name(declaration.Name(), typ).run(context)
	if err != nil {
		return nil, err
	}
	if _, err = markBound(context, nameExpr); err != nil {
		return nil, err
	}
	return builder.typed.LetIn(bodyExpr, declaration)
}

type ExceptUpdate struct {
	Argument    Expr
	Replacement Expr
}

func (builder *TypedBuilder) ExceptMany(function Expr, updates ...ExceptUpdate) (Expr, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("ExceptMany requires at least one update")
	}
	result := function
	for _, update := range updates {
		var err error
		result, err = builder.Except(result, update.Argument, update.Replacement)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

type ScopedExceptUpdate struct {
	Argument    ExprInstruction
	Replacement ExprInstruction
}

func (builder *ScopedBuilder) ExceptMany(function ExprInstruction, updates ...ScopedExceptUpdate) ExprInstruction {
	if len(updates) == 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("ExceptMany requires at least one update")
		}}
	}
	result := function
	for _, update := range updates {
		result = builder.Except(result, update.Argument, update.Replacement)
	}
	return result
}

func (builder *TypedBuilder) ExceptDeep(function, replacement Expr, arguments ...Expr) (Expr, error) {
	if len(arguments) == 0 {
		return nil, fmt.Errorf("ExceptDeep requires at least one argument")
	}
	if len(arguments) == 1 {
		return builder.Except(function, arguments[0], replacement)
	}
	nestedFunction, err := builder.App(function, arguments[0])
	if err != nil {
		return nil, err
	}
	nested, err := builder.ExceptDeep(nestedFunction, replacement, arguments[1:]...)
	if err != nil {
		return nil, err
	}
	return builder.Except(function, arguments[0], nested)
}

func (builder *ScopedBuilder) ExceptDeep(function, replacement ExprInstruction, arguments ...ExprInstruction) ExprInstruction {
	if len(arguments) == 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("ExceptDeep requires at least one argument")
		}}
	}
	if len(arguments) == 1 {
		return builder.Except(function, arguments[0], replacement)
	}
	return builder.Except(function, arguments[0],
		builder.ExceptDeep(builder.App(function, arguments[0]), replacement, arguments[1:]...))
}

type ExceptPath struct {
	Replacement Expr
	Arguments   []Expr
}

func (builder *TypedBuilder) ExceptGeneral(function Expr, paths ...ExceptPath) (Expr, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("ExceptGeneral requires at least one path")
	}
	depth := len(paths[0].Arguments)
	if depth == 0 {
		return nil, fmt.Errorf("ExceptGeneral paths must be nonempty")
	}
	result := function
	for _, path := range paths {
		if len(path.Arguments) != depth {
			return nil, fmt.Errorf("ExceptGeneral paths must have uniform depth")
		}
		var err error
		result, err = builder.ExceptDeep(result, path.Replacement, path.Arguments...)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

type ScopedExceptPath struct {
	Replacement ExprInstruction
	Arguments   []ExprInstruction
}

func (builder *ScopedBuilder) ExceptGeneral(function ExprInstruction, paths ...ScopedExceptPath) ExprInstruction {
	if len(paths) == 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("ExceptGeneral requires at least one path")
		}}
	}
	depth := len(paths[0].Arguments)
	if depth == 0 {
		return ExprInstruction{run: func(*scopeContext) (Expr, error) {
			return nil, fmt.Errorf("ExceptGeneral paths must be nonempty")
		}}
	}
	result := function
	for _, path := range paths {
		if len(path.Arguments) != depth {
			return ExprInstruction{run: func(*scopeContext) (Expr, error) {
				return nil, fmt.Errorf("ExceptGeneral paths must have uniform depth")
			}}
		}
		result = builder.ExceptDeep(result, path.Replacement, path.Arguments...)
	}
	return result
}

func (builder *TypedBuilder) VarDeclAsName(declaration *VarDecl) (*NameExpr, error) {
	if declaration == nil {
		return nil, fmt.Errorf("variable declaration is nil")
	}
	typ, ok := declaration.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("variable declaration %s is untyped", declaration.Name())
	}
	return builder.Name(declaration.Name(), typ), nil
}

func (builder *ScopedBuilder) VarDeclAsName(declaration *VarDecl) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		name, err := builder.typed.VarDeclAsName(declaration)
		if err != nil {
			return nil, err
		}
		typ, _ := name.Tag().Type()
		return builder.Name(name.Name(), typ).run(context)
	}}
}

func (builder *TypedBuilder) VarDeclAsNameEx(declaration *VarDecl) (*NameExpr, error) {
	return builder.VarDeclAsName(declaration)
}

func (builder *ScopedBuilder) VarDeclAsNameEx(declaration *VarDecl) ExprInstruction {
	return builder.VarDeclAsName(declaration)
}
