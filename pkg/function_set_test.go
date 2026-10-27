package apalache

import (
	"context"
	"testing"
)

func TestStratifiedFunctionSetEmptyOperands(t *testing.T) {
	intSetType := mustSet(IntT1)
	boolSetType := mustSet(BoolT1)
	functionType := mustFun(IntT1, BoolT1)
	functionSetType := mustSet(functionType)
	emptyDomain := MustOperExpr(Op("SET_ENUM"), Typed(intSetType))
	emptyCodomain := MustOperExpr(Op("SET_ENUM"), Typed(boolSetType))
	nonEmptyDomain := MustOperExpr(Op("SET_ENUM"), Typed(intSetType),
		NewValueExpr(Int64Value(1), Typed(IntT1)))
	nonEmptyCodomain := MustOperExpr(Op("SET_ENUM"), Typed(boolSetType),
		NewValueExpr(BoolValue(true), Typed(BoolT1)))
	tests := []struct {
		name             string
		domain, codomain Expr
		wantElements     int
	}{
		{"empty domain and codomain", emptyDomain, emptyCodomain, 1},
		{"empty domain", emptyDomain, nonEmptyCodomain, 1},
		{"empty codomain", nonEmptyDomain, emptyCodomain, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			expression := MustOperExpr(Op("FUN_SET"), Typed(functionSetType), test.domain, test.codomain)
			scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			if _, ordinary := result.Type.(BasicCellType); !ordinary {
				t.Fatalf("degenerate function set type = %T", result.Type)
			}
			pointers := scope.Arena.GetHas(result)
			if len(pointers) != test.wantElements {
				t.Fatalf("function set elements = %#v, want %d", pointers, test.wantElements)
			}
			if len(pointers) == 1 {
				domain, domainErr := scope.Arena.Domain(pointers[0].Element())
				if domainErr != nil || len(scope.Arena.GetHas(domain)) != 0 {
					t.Fatalf("canonical empty function domain = %v, %v", domain, domainErr)
				}
			}
		})
	}
}

func TestStratifiedFunctionSetRetainsOperands(t *testing.T) {
	intSetType := mustSet(IntT1)
	boolSetType := mustSet(BoolT1)
	functionType := mustFun(IntT1, BoolT1)
	expression := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType), NewValueExpr(Int64Value(1), Typed(IntT1))),
		MustOperExpr(Op("SET_ENUM"), Typed(boolSetType), NewValueExpr(BoolValue(false), Typed(BoolT1))))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Type.(FiniteFunctionSetCellType); !ok {
		t.Fatalf("function-set arena type = %T", result.Type)
	}
	domain, domainErr := scope.Arena.Domain(result)
	codomain, codomainErr := scope.Arena.Codomain(result)
	if domainErr != nil || codomainErr != nil || len(scope.Arena.GetHas(domain)) != 1 || len(scope.Arena.GetHas(codomain)) != 1 {
		t.Fatalf("function-set operands = %v/%v, errors %v/%v", domain, codomain, domainErr, codomainErr)
	}
}

func TestStratifiedFunctionSetSymbolicEmptiness(t *testing.T) {
	intSetType := mustSet(IntT1)
	boolSetType := mustSet(BoolT1)
	functionType := mustFun(IntT1, BoolT1)
	predicate := NewNameExpr("p", Typed(BoolT1))
	domain := MustOperExpr(Op("SET_FILTER"), Typed(intSetType),
		NewNameExpr("x", Typed(IntT1)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType), NewValueExpr(Int64Value(1), Typed(IntT1))), predicate)
	emptyCodomain := MustOperExpr(Op("SET_ENUM"), Typed(boolSetType))
	expression := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)), domain, emptyCodomain)

	for _, pValue := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty domain", true: "nonempty domain"}[pValue], func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope := InitialRewriterScope()
			newArena, pCell := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
			scope.Arena, scope.Binding = newArena, scope.Binding.With("p", pCell)
			scope, result, err := rewriter.Rewrite(expression, scope)
			if err != nil {
				t.Fatal(err)
			}
			pointers := scope.Arena.GetHas(result)
			if len(pointers) != 1 {
				t.Fatalf("symbolic function set pointers = %#v", pointers)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			var pAssertion Expr = pCell.NameExpr()
			if !pValue {
				pAssertion = MustOperExpr(Op("NOT"), Typed(BoolT1), pAssertion)
			}
			if err := encoder.Assert(pAssertion); err != nil {
				t.Fatal(err)
			}
			membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), pointers[0].Element().NameExpr(), result.NameExpr())
			if pValue {
				membership = MustOperExpr(Op("NOT"), Typed(BoolT1), membership)
			}
			if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), membership)); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("symbolic function-set emptiness = %v, %v", status, err)
			}
		})
	}
}
