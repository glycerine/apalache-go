package apalache

// ExpressionRewriter returns (replacement, true) when an expression should be
// replaced and (_, false) otherwise.
type ExpressionRewriter func(Expr) (Expr, bool)

// ReplaceEqual returns a bottom-up transformation that replaces every
// structurally equal occurrence with a freshly constructed expression.
func ReplaceEqual(tracker TransformationTracker, pattern Expr, replacement func() Expr) ExprTransform {
	return ReplaceMatching(tracker, func(expression Expr) bool {
		return EqualExpr(pattern, expression)
	}, replacement)
}

// ReplaceMatching returns a bottom-up transformation. Children are rewritten
// before the predicate is tested on their potentially reconstructed parent.
func ReplaceMatching(tracker TransformationTracker, predicate func(Expr) bool, replacement func() Expr) ExprTransform {
	return RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		if !predicate(expression) {
			return nil, false
		}
		return replacement(), true
	})
}

// RewriteExpressions applies rewrite bottom-up throughout operators and
// LET-IN declarations.
func RewriteExpressions(tracker TransformationTracker, rewrite ExpressionRewriter) ExprTransform {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var transform ExprTransform
	transform = tracker.TrackExpression(func(expression Expr) Expr {
		var rebuilt Expr = expression
		switch typed := expression.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			changed := false
			for index, argument := range arguments {
				transformed := transform(argument)
				arguments[index] = transformed
				changed = changed || !EqualExpr(argument, transformed)
			}
			if changed {
				rebuilt = MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
		case *LetInExpr:
			declarations := typed.Declarations()
			changed := false
			for index, declaration := range declarations {
				originalBody := declaration.Body()
				newBody := transform(originalBody)
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(newBody)
				declarations[index] = tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
				changed = changed || !EqualExpr(originalBody, newBody)
			}
			body := transform(typed.Body())
			changed = changed || !EqualExpr(typed.Body(), body)
			if changed {
				rebuilt = NewLetInExpr(body, typed.Tag(), declarations...)
			}
		}
		if replacement, ok := rewrite(rebuilt); ok {
			if replacement == nil {
				panic("expression rewriter returned a nil replacement")
			}
			return replacement
		}
		return rebuilt
	})
	return transform
}

// FlattenBooleanConnectives flattens nested AND and OR expressions while
// retaining argument order and every reconstructed node's original tag.
func FlattenBooleanConnectives(tracker TransformationTracker) ExprTransform {
	return RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		operatorExpression, ok := expression.(*OperExpr)
		if !ok {
			return nil, false
		}
		name := operatorExpression.Operator().Name()
		if name != "AND" && name != "OR" {
			return nil, false
		}
		arguments := operatorExpression.Arguments()
		flattened := make([]Expr, 0, len(arguments))
		changed := false
		for _, argument := range arguments {
			child, sameKind := argument.(*OperExpr)
			if sameKind && child.Operator().Name() == name {
				flattened = append(flattened, child.Arguments()...)
				changed = true
			} else {
				flattened = append(flattened, argument)
			}
		}
		if !changed {
			return nil, false
		}
		return MustOperExpr(operatorExpression.Operator(), operatorExpression.Tag(), flattened...), true
	})
}

// PropagatePrimes pushes PRIME through expressions until it reaches state
// variable names. Primes over constants and values disappear.
func PropagatePrimes(tracker TransformationTracker, stateVariables map[string]struct{}) ExprTransform {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var transform func(Expr, bool) Expr
	transform = func(expression Expr, prime bool) Expr {
		tracked := tracker.TrackExpression(func(expression Expr) Expr {
			switch typed := expression.(type) {
			case *OperExpr:
				arguments := typed.Arguments()
				if typed.Operator().Name() == "PRIME" {
					return transform(arguments[0], true)
				}
				for index, argument := range arguments {
					arguments[index] = transform(argument, prime)
				}
				return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			case *NameExpr:
				if _, isStateVariable := stateVariables[typed.Name()]; prime && isStateVariable {
					return MustOperExpr(Op("PRIME"), typed.Tag(), typed)
				}
				return typed
			case *LetInExpr:
				declarations := typed.Declarations()
				changed := false
				for index, declaration := range declarations {
					body := transform(declaration.Body(), prime)
					copy := declaration.copyWithTag(declaration.Tag())
					copy.SetBody(body)
					declarations[index] = tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
					changed = changed || !EqualExpr(body, declaration.Body())
				}
				body := transform(typed.Body(), prime)
				if !changed && EqualExpr(body, typed.Body()) {
					return typed
				}
				return NewLetInExpr(body, typed.Tag(), declarations...)
			default:
				return expression
			}
		})
		return tracked(expression)
	}
	return func(expression Expr) Expr { return transform(expression, false) }
}

// DecorateWithPrime primes the selected names without adding a second prime
// to names that are already primed.
func DecorateWithPrime(tracker TransformationTracker, variables map[string]struct{}) ExprTransform {
	return RewriteExpressions(tracker, func(expression Expr) (Expr, bool) {
		switch typed := expression.(type) {
		case *NameExpr:
			if _, found := variables[typed.Name()]; found {
				return MustOperExpr(Op("PRIME"), typed.Tag(), typed), true
			}
		case *OperExpr:
			if typed.Operator().Name() == "PRIME" {
				argument := typed.Arguments()[0]
				if once, ok := argument.(*OperExpr); ok && once.Operator().Name() == "PRIME" {
					return once, true
				}
			}
		}
		return nil, false
	})
}

// TransformModuleExpressions applies a transformation to operator and
// assumption bodies selected by include.
func TransformModuleExpressions(module *Module, transform ExprTransform, include func(Decl) bool) *Module {
	declarations := module.Declarations()
	for index, declaration := range declarations {
		if include != nil && !include(declaration) {
			continue
		}
		switch typed := declaration.(type) {
		case *OperDecl:
			body := transform(typed.Body())
			if body.ID() != typed.Body().ID() {
				copy := typed.copyWithTag(typed.Tag())
				copy.SetBody(body)
				declarations[index] = copy
			}
		case *AssumeDecl:
			body := transform(typed.Body())
			if body.ID() != typed.Body().ID() {
				name, named := typed.DefinedName()
				if named {
					declarations[index] = NewNamedAssumeDecl(name, body, typed.Tag())
				} else {
					declarations[index] = NewUnnamedAssumeDecl(body, typed.Tag())
				}
			}
		}
	}
	result := NewModule(module.Name(), declarations)
	return result
}

// UnrollSimpleUnchanged rewrites UNCHANGED e to e' = e and a top-level tuple
// to the conjunction of element-wise equalities. It deliberately does not
// descend into LET-IN expressions, matching the post-inlining ReTLA pass.
func UnrollSimpleUnchanged(tracker TransformationTracker) ExprTransform {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	primeEquality := func(expression Expr) Expr {
		// Scala case-class copy creates a fresh root while retaining the
		// original children. WithTag with the existing tag has that exact
		// shallow-copy behavior for every non-null expression variant.
		leftValue := expression.WithTag(expression.Tag())
		rightValue := expression.WithTag(expression.Tag())
		left := MustOperExpr(Op("PRIME"), expression.Tag(), leftValue)
		return MustOperExpr(Op("EQ"), Typed(BoolT1), left, rightValue)
	}
	var transform ExprTransform
	transform = tracker.TrackExpression(func(expression Expr) Expr {
		operatorExpression, ok := expression.(*OperExpr)
		if !ok {
			return expression
		}
		arguments := operatorExpression.Arguments()
		if operatorExpression.Operator().Name() == "UNCHANGED" {
			argument := arguments[0]
			if tuple, isTuple := argument.(*OperExpr); isTuple && tuple.Operator().Name() == "TUPLE" {
				equalities := tuple.Arguments()
				for index, element := range equalities {
					equalities[index] = primeEquality(element)
				}
				return MustOperExpr(Op("AND"), Typed(BoolT1), equalities...)
			}
			return primeEquality(argument)
		}
		changed := false
		for index, argument := range arguments {
			arguments[index] = transform(argument)
			changed = changed || !EqualExpr(argument, arguments[index])
		}
		if !changed {
			return operatorExpression
		}
		return MustOperExpr(operatorExpression.Operator(), operatorExpression.Tag(), arguments...)
	})
	return transform
}
