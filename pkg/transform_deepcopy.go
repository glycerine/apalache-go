package apalache

// DeepCopier constructs structurally identical IR with fresh IDs and reports
// every copied node through its tracker.
type DeepCopier struct{ tracker TransformationTracker }

func NewDeepCopier(tracker TransformationTracker) *DeepCopier {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &DeepCopier{tracker: tracker}
}

func (copier *DeepCopier) Expression(expr Expr) Expr {
	return copier.tracker.TrackExpression(copier.copyExpression)(expr)
}

func (copier *DeepCopier) copyExpression(expr Expr) Expr {
	switch typed := expr.(type) {
	case *nullExpr:
		return NullExpr()
	case *ValueExpr:
		return NewValueExpr(typed.Value(), typed.Tag())
	case *NameExpr:
		return NewNameExpr(typed.Name(), typed.Tag())
	case *OperExpr:
		arguments := typed.Arguments()
		for index, argument := range arguments {
			arguments[index] = copier.Expression(argument)
		}
		return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
	case *LetInExpr:
		declarations := typed.Declarations()
		for index, declaration := range declarations {
			declarations[index] = copier.Declaration(declaration).(*OperDecl)
		}
		return NewLetInExpr(copier.Expression(typed.Body()), typed.Tag(), declarations...)
	default:
		panic("unknown expression implementation")
	}
}

func (copier *DeepCopier) Declaration(declaration Decl) Decl {
	return copier.tracker.TrackDeclaration(copier.copyDeclaration)(declaration)
}

func (copier *DeepCopier) copyDeclaration(declaration Decl) Decl {
	switch typed := declaration.(type) {
	case *ConstDecl:
		return NewConstDecl(typed.Name(), typed.Tag())
	case *VarDecl:
		result := NewVarDecl(typed.Name(), typed.Tag())
		if typed.IsInternal() {
			result.MarkInternal()
		}
		return result
	case *AssumeDecl:
		name, named := typed.DefinedName()
		if named {
			return NewNamedAssumeDecl(name, copier.Expression(typed.Body()), typed.Tag())
		}
		return NewUnnamedAssumeDecl(copier.Expression(typed.Body()), typed.Tag())
	case *OperDecl:
		result := NewOperDecl(typed.Name(), typed.FormalParams(), copier.Expression(typed.Body()), typed.Tag())
		result.SetRecursive(typed.IsRecursive())
		return result
	case *TheoremDecl:
		return NewTheoremDecl(typed.Name(), copier.Expression(typed.Body()), typed.Tag())
	default:
		panic("unknown declaration implementation")
	}
}

func (copier *DeepCopier) Module(module *Module) *Module {
	declarations := module.Declarations()
	for index, declaration := range declarations {
		declarations[index] = copier.Declaration(declaration)
	}
	return NewModule(module.Name(), declarations)
}
