package apalache

// OptimizeIntegerQuantifier rewrites one bounded integer quantifier into the
// unbounded form consumed by symbolic rewriting. Like the Scala pass, this is
// deliberately shallow; pipeline traversal controls where it is applied.
func OptimizeIntegerQuantifier(expression Expr, tracker TransformationTracker) Expr {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return tracker.TrackExpression(func(original Expr) Expr {
		quantifier, ok := original.(*OperExpr)
		if !ok || (quantifier.Operator().Name() != "FORALL3" && quantifier.Operator().Name() != "EXISTS3") {
			return original
		}
		arguments := quantifier.Arguments()
		bound, ok := arguments[0].(*NameExpr)
		if !ok || !allowedIntegerSet(arguments[1]) {
			return original
		}
		predicate := arguments[2]
		outer := "AND"
		unbounded := "EXISTS2"
		if quantifier.Operator().Name() == "FORALL3" {
			outer, unbounded = "IMPLIES", "FORALL2"
		}
		switch set := arguments[1].(type) {
		case *ValueExpr:
			if predefined, _ := set.Value().(PredefinedSet); predefined == NatSet {
				guard := MustOperExpr(Op("GE"), Typed(BoolT1), NewNameExpr(bound.Name(), Typed(IntT1)), NewValueExpr(Int64Value(0), Typed(IntT1)))
				predicate = MustOperExpr(Op(outer), Typed(BoolT1), guard, predicate)
			}
		case *OperExpr:
			rangeArguments := set.Arguments()
			lower := MustOperExpr(Op("LE"), Typed(BoolT1), rangeArguments[0], NewNameExpr(bound.Name(), Typed(IntT1)))
			upper := MustOperExpr(Op("LE"), Typed(BoolT1), NewNameExpr(bound.Name(), Typed(IntT1)), rangeArguments[1])
			guard := MustOperExpr(Op("AND"), Typed(BoolT1), lower, upper)
			predicate = MustOperExpr(Op(outer), Typed(BoolT1), guard, predicate)
		}
		return MustOperExpr(Op(unbounded), Typed(BoolT1), bound, predicate)
	})(expression)
}

func allowedIntegerSet(expression Expr) bool {
	if value, ok := expression.(*ValueExpr); ok {
		predefined, ok := value.Value().(PredefinedSet)
		return ok && (predefined == IntSet || predefined == NatSet)
	}
	operation, ok := expression.(*OperExpr)
	return ok && operation.Operator().Name() == "INT_RANGE"
}
