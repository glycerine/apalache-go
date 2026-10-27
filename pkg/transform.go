package apalache

type ExprTransform func(Expr) Expr
type DeclTransform func(Decl) Decl
type OperDeclTransform func(*OperDecl) *OperDecl

// TouchExpr distinguishes an unchanged expression from one produced by a
// transformation, matching Scala's Left(original)/Right(changed) convention.
type TouchExpr struct {
	Expr    Expr
	Changed bool
}

type TouchTransform func(TouchExpr) TouchExpr

type TransformationListener interface {
	OnExpressionTransformation(original, transformed Expr)
	OnDeclarationTransformation(original, transformed Decl)
}

type TransformationTracker interface {
	TrackExpression(ExprTransform) ExprTransform
	TrackTouch(TouchTransform) TouchTransform
	TrackDeclaration(DeclTransform) DeclTransform
	TrackOperatorDeclaration(OperDeclTransform) OperDeclTransform
	Hold(from, to Expr) Expr
}

type IdleTracker struct{}

func (IdleTracker) TrackExpression(transform ExprTransform) ExprTransform { return transform }
func (IdleTracker) TrackTouch(transform TouchTransform) TouchTransform    { return transform }
func (IdleTracker) TrackDeclaration(transform DeclTransform) DeclTransform {
	return transform
}
func (IdleTracker) TrackOperatorDeclaration(transform OperDeclTransform) OperDeclTransform {
	return transform
}
func (tracker IdleTracker) Hold(_ Expr, to Expr) Expr { return to }

type ListenerTracker struct{ listeners []TransformationListener }

func NewListenerTracker(listeners ...TransformationListener) *ListenerTracker {
	return &ListenerTracker{listeners: append([]TransformationListener(nil), listeners...)}
}
func (tracker *ListenerTracker) TrackExpression(transform ExprTransform) ExprTransform {
	return func(original Expr) Expr {
		transformed := transform(original)
		for _, listener := range tracker.listeners {
			listener.OnExpressionTransformation(original, transformed)
		}
		return transformed
	}
}
func (tracker *ListenerTracker) TrackTouch(transform TouchTransform) TouchTransform {
	return func(input TouchExpr) TouchExpr {
		if !input.Changed {
			return input
		}
		result := transform(input)
		if !result.Changed {
			return TouchExpr{Expr: input.Expr}
		}
		for _, listener := range tracker.listeners {
			listener.OnExpressionTransformation(input.Expr, result.Expr)
		}
		return result
	}
}
func (tracker *ListenerTracker) TrackDeclaration(transform DeclTransform) DeclTransform {
	return func(original Decl) Decl {
		transformed := transform(original)
		for _, listener := range tracker.listeners {
			listener.OnDeclarationTransformation(original, transformed)
		}
		return transformed
	}
}
func (tracker *ListenerTracker) TrackOperatorDeclaration(transform OperDeclTransform) OperDeclTransform {
	return func(original *OperDecl) *OperDecl {
		transformed := transform(original)
		for _, listener := range tracker.listeners {
			listener.OnDeclarationTransformation(original, transformed)
		}
		return transformed
	}
}
func (tracker *ListenerTracker) Hold(from, to Expr) Expr {
	return tracker.TrackExpression(func(Expr) Expr { return to })(from)
}

// DeclarationTransformFromExpression adapts an expression transform to
// operator declaration bodies and tracks both levels of change.
func DeclarationTransformFromExpression(tracker TransformationTracker, transform ExprTransform) DeclTransform {
	return tracker.TrackDeclaration(func(declaration Decl) Decl {
		operator, ok := declaration.(*OperDecl)
		if !ok {
			return declaration
		}
		copy := operator.copyWithTag(operator.Tag())
		copy.SetBody(transform(operator.Body()))
		return copy
	})
}
