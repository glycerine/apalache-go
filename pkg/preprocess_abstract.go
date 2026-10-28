package apalache

// PartialExprTransform is an ordered shallow rewrite rule. The Boolean result
// reports whether the rule is defined for the supplied expression.
type PartialExprTransform func(Expr) (Expr, bool)

// AbstractTransformer mirrors Scala's AbstractTransformer: it recursively
// rewrites operator arguments and LET bodies, then applies the first matching
// shallow rule to operators and leaves. LET expressions themselves are not
// offered to shallow rules.
type AbstractTransformer struct {
	tracker TransformationTracker
	rules   []PartialExprTransform
}

func NewAbstractTransformer(tracker TransformationTracker, rules ...PartialExprTransform) *AbstractTransformer {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &AbstractTransformer{
		tracker: tracker,
		rules:   append([]PartialExprTransform(nil), rules...),
	}
}

// Transform recursively applies the configured rules from left to right.
func (transformer *AbstractTransformer) Transform(expression Expr) Expr {
	var transform ExprTransform
	transform = transformer.tracker.TrackExpression(func(current Expr) Expr {
		switch typed := current.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			changed := false
			for index, argument := range arguments {
				arguments[index] = transform(argument)
				changed = changed || arguments[index].ID() != argument.ID()
			}
			var shallow Expr = typed
			if changed {
				shallow = transformer.tracker.Hold(typed, MustOperExpr(typed.Operator(), typed.Tag(), arguments...))
			}
			return transformer.transformOneLevel(shallow)
		case *LetInExpr:
			declarations := typed.Declarations()
			for index, declaration := range declarations {
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(transform(declaration.Body()))
				declarations[index] = copy
			}
			return NewLetInExpr(transform(typed.Body()), typed.Tag(), declarations...)
		default:
			return transformer.transformOneLevel(current)
		}
	})
	return transform(expression)
}

func (transformer *AbstractTransformer) transformOneLevel(expression Expr) Expr {
	return transformer.tracker.TrackExpression(func(current Expr) Expr {
		for _, rule := range transformer.rules {
			if transformed, defined := rule(current); defined {
				return transformed
			}
		}
		return current
	})(expression)
}
