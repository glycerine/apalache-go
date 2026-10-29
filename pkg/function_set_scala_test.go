package apalache

import "testing"

func TestStratifiedFunctionSetScalaOracleCases(t *testing.T) {
	set := func(elementType Type, elements ...Expr) Expr {
		return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(elementType)), elements...)
	}
	intSet := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intLiteral(value)
		}
		return set(IntT1, elements...)
	}
	boolSet := func(values ...bool) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = boolExpr(value)
		}
		return set(BoolT1, elements...)
	}
	functionSet := func(argument, result Type, domain, codomain Expr) Expr {
		return MustOperExpr(Op("FUN_SET"), Typed(mustSet(mustFun(argument, result))), domain, codomain)
	}
	function := func(argument, result Type, binder string, domain, body Expr) Expr {
		return MustOperExpr(Op("FUN_CTOR"), Typed(mustFun(argument, result)), body,
			NewNameExpr(binder, Typed(argument)), domain)
	}
	equal := func(left, right Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), left, right)
	}
	check := func(t *testing.T, expression Expr, want bool) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, result, want)
	}
	skolem := func(binder *NameExpr, domain, predicate Expr) Expr {
		exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, domain, predicate)
		return MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists)
	}
	assignment := func(name string, typ Type, value Expr) Expr {
		prime := MustOperExpr(Op("PRIME"), Typed(typ), NewNameExpr(name, Typed(typ)))
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, value)
	}

	t.Run("separate empty function sets are equal", func(t *testing.T) {
		left := functionSet(IntT1, IntT1, intSet(), intSet())
		right := functionSet(IntT1, IntT1, intSet(), intSet())
		check(t, equal(left, right), true)
	})

	t.Run("finite function set retains exact domain and codomain", func(t *testing.T) {
		expression := functionSet(IntT1, BoolT1, intSet(1, 2, 3), boolSet(false, true))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		domain, domainErr := scope.Arena.Domain(result)
		codomain, codomainErr := scope.Arena.Codomain(result)
		if domainErr != nil || codomainErr != nil || len(scope.Arena.GetHas(domain)) != 3 ||
			len(scope.Arena.GetHas(codomain)) != 2 {
			t.Fatalf("function-set operands = %v/%v, errors %v/%v", domain, codomain, domainErr, codomainErr)
		}
	})

	t.Run("expanded powerset is retained as compact codomain", func(t *testing.T) {
		setBool := mustSet(BoolT1)
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false, true))
		expanded := MustOperExpr(Op("Apalache!Expand"), Typed(mustSet(setBool)), power)
		expression := functionSet(IntT1, setBool, intSet(1, 2), expanded)
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		domain, domainErr := scope.Arena.Domain(result)
		codomain, codomainErr := scope.Arena.Codomain(result)
		if domainErr != nil || codomainErr != nil || len(scope.Arena.GetHas(domain)) != 2 ||
			len(scope.Arena.GetHas(codomain)) != 4 {
			t.Fatalf("expanded function-set operands = %v/%v, errors %v/%v", domain, codomain, domainErr, codomainErr)
		}
	})

	t.Run("Skolem assignment from powerset function set", func(t *testing.T) {
		setBool := mustSet(BoolT1)
		functionType := mustFun(IntT1, setBool)
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false, true))
		domain := functionSet(IntT1, setBool, intSet(1, 2), power)
		binder := NewNameExpr("f", Typed(functionType))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(
			skolem(binder, domain, assignment("g", functionType, binder)), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		assigned, found := scope.Binding.Get("g'")
		if !found || !EqualTypes(assigned.Type.TLAType(), functionType) {
			t.Fatalf("powerset function assignment = %v, %v", assigned, found)
		}
		checkRewrittenTruth(t, rewriter, scope, result, true)
	})

	t.Run("duplicate domain expressions select one consistent value", func(t *testing.T) {
		setBool := mustSet(BoolT1)
		functionType := mustFun(IntT1, setBool)
		oneViaSubtraction := MustOperExpr(Op("MINUS"), Typed(IntT1), intLiteral(2), intLiteral(1))
		domainValues := set(IntT1, intLiteral(1), oneViaSubtraction, intLiteral(2))
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false, true))
		binder := NewNameExpr("f", Typed(functionType))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(skolem(binder,
			functionSet(IntT1, setBool, domainValues, power), assignment("g", functionType, binder)),
			InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		assigned, found := scope.Binding.Get("g'")
		if !found {
			t.Fatal("duplicate-domain Skolem assignment dropped g'")
		}
		first := MustOperExpr(Op("FUN_APP"), Typed(setBool), assigned.NameExpr(), oneViaSubtraction)
		secondIndex := MustOperExpr(Op("MINUS"), Typed(IntT1), intLiteral(3), intLiteral(2))
		second := MustOperExpr(Op("FUN_APP"), Typed(setBool), assigned.NameExpr(), secondIndex)
		scope, claim, err := rewriter.Rewrite(booleanAnd(result.NameExpr(), equal(first, second)), scope)
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, claim, true)
	})

	t.Run("impossible powerset result makes Skolem false", func(t *testing.T) {
		setBool := mustSet(BoolT1)
		functionType := mustFun(IntT1, setBool)
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false))
		binder := NewNameExpr("f", Typed(functionType))
		application := MustOperExpr(Op("FUN_APP"), Typed(setBool), binder, intLiteral(1))
		predicate := equal(application, boolSet(true))
		check(t, skolem(binder, functionSet(IntT1, setBool, intSet(1, 2), power), predicate), false)
	})

	t.Run("nested function-set Skolem assignment remains compact", func(t *testing.T) {
		setBool := mustSet(BoolT1)
		innerType := mustFun(IntT1, setBool)
		outerType := mustFun(IntT1, innerType)
		power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false, true))
		innerSet := functionSet(IntT1, setBool, intSet(3), power)
		outerSet := functionSet(IntT1, innerType, intSet(1, 2), innerSet)
		binder := NewNameExpr("f", Typed(outerType))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(
			skolem(binder, outerSet, assignment("g", outerType, binder)), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		assigned, found := scope.Binding.Get("g'")
		if !found || !EqualTypes(assigned.Type.TLAType(), outerType) {
			t.Fatalf("nested function assignment = %v, %v", assigned, found)
		}
		checkRewrittenTruth(t, rewriter, scope, result, true)
	})

	t.Run("function membership honors domain and powerset codomain", func(t *testing.T) {
		setBool := mustSet(BoolT1)
		domain := intSet(1, 2)
		powerFalse := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false))
		powerBoth := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setBool)), boolSet(false, true))
		x := NewNameExpr("x", Typed(IntT1))
		body := boolSet(true)
		candidate := function(IntT1, setBool, "x", domain, body)
		check(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), candidate,
			functionSet(IntT1, setBool, domain, powerFalse)), false)
		check(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), candidate,
			functionSet(IntT1, setBool, domain, powerBoth)), true)

		predicateBody := set(BoolT1, equal(x, intLiteral(1)))
		predicateFunction := function(IntT1, setBool, "x", domain, predicateBody)
		check(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), predicateFunction,
			functionSet(IntT1, setBool, domain, powerBoth)), true)
	})

	t.Run("filtered and integer-range domains retain membership and size", func(t *testing.T) {
		filterBinder := NewNameExpr("i", Typed(IntT1))
		filtered := MustOperExpr(Op("SET_FILTER"), Typed(mustSet(IntT1)), filterBinder, intSet(0, 1, 2),
			MustOperExpr(Op("NE"), Typed(BoolT1), filterBinder, intLiteral(0)))
		candidate := function(IntT1, IntT1, "x", filtered, intLiteral(3))
		funSet := functionSet(IntT1, IntT1, intSet(1, 2), intSet(3, 4))
		check(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), candidate, funSet), true)

		rangeDomain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(0),
			MustOperExpr(Op("MINUS"), Typed(IntT1), intLiteral(5), intLiteral(1)))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(
			functionSet(IntT1, BoolT1, rangeDomain, boolSet(false, true)), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		domainCell, err := scope.Arena.Domain(result)
		if err != nil || len(scope.Arena.GetHas(domainCell)) != 5 {
			t.Fatalf("range function-set domain = %v, %v", domainCell, err)
		}
	})
}
