package apalache

import "testing"

func TestTypedArithmeticBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	tests := []struct {
		name     string
		operator string
		want     Type
		build    func(Expr, Expr) (*OperExpr, error)
	}{
		{"plus", "PLUS", IntT1, builder.Plus},
		{"minus", "MINUS", IntT1, builder.Minus},
		{"mult", "MULT", IntT1, builder.Mult},
		{"div", "DIV", IntT1, builder.Div},
		{"mod", "MOD", IntT1, builder.Mod},
		{"exp", "POW", IntT1, builder.Exp},
		{"dotdot", "INT_RANGE", mustSetType(t, IntT1), builder.DotDot},
		{"lt", "LT", BoolT1, builder.Lt},
		{"gt", "GT", BoolT1, builder.Gt},
		{"le", "LE", BoolT1, builder.Le},
		{"ge", "GE", BoolT1, builder.Ge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left := builder.Name("x", IntT1)
			right := builder.Name("y", IntT1)
			result, err := test.build(left, right)
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, result, test.want)
			arguments := result.Arguments()
			if result.Operator().Name() != test.operator || len(arguments) != 2 || arguments[0] != left || arguments[1] != right {
				t.Fatalf("result = %v(%v), want %s(x, y)", result.Operator(), arguments, test.operator)
			}
			if _, err = test.build(left, builder.Name("bad", StrT1)); err == nil {
				t.Fatal("accepted an ill-typed right operand")
			}
			if _, err = test.build(builder.Name("bad", StrT1), right); err == nil {
				t.Fatal("accepted an ill-typed left operand")
			}
		})
	}

	operand := builder.Name("x", IntT1)
	result, err := builder.UMinus(operand)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, result, IntT1)
	arguments := result.Arguments()
	if result.Operator().Name() != "UNARY_MINUS" || len(arguments) != 1 || arguments[0] != operand {
		t.Fatalf("unary minus = %v(%v)", result.Operator(), arguments)
	}
	if _, err = builder.UMinus(builder.Name("bad", StrT1)); err == nil {
		t.Fatal("accepted an ill-typed unary operand")
	}
}

func TestScopedArithmeticBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	tests := []struct {
		name     string
		operator string
		want     Type
		build    func(ExprInstruction, ExprInstruction) ExprInstruction
	}{
		{"plus", "PLUS", IntT1, builder.Plus},
		{"minus", "MINUS", IntT1, builder.Minus},
		{"mult", "MULT", IntT1, builder.Mult},
		{"div", "DIV", IntT1, builder.Div},
		{"mod", "MOD", IntT1, builder.Mod},
		{"exp", "POW", IntT1, builder.Exp},
		{"dotdot", "INT_RANGE", mustSetType(t, IntT1), builder.DotDot},
		{"lt", "LT", BoolT1, builder.Lt},
		{"gt", "GT", BoolT1, builder.Gt},
		{"le", "LE", BoolT1, builder.Le},
		{"ge", "GE", BoolT1, builder.Ge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.build(builder.Name("x", IntT1), builder.Name("y", IntT1)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, result, test.want)
			operation := result.(*OperExpr)
			if operation.Operator().Name() != test.operator {
				t.Fatalf("operator = %s, want %s", operation.Operator(), test.operator)
			}
			if _, err = test.build(builder.Name("x", IntT1), builder.Name("bad", StrT1)).Build(); err == nil {
				t.Fatal("accepted an ill-typed right operand")
			}
			if _, err = test.build(builder.Name("bad", StrT1), builder.Name("y", IntT1)).Build(); err == nil {
				t.Fatal("accepted an ill-typed left operand")
			}
		})
	}

	result, err := builder.UMinus(builder.Name("x", IntT1)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, result, IntT1)
	if result.(*OperExpr).Operator().Name() != "UNARY_MINUS" {
		t.Fatalf("unary operator = %s", result.(*OperExpr).Operator())
	}
	if _, err = builder.UMinus(builder.Name("bad", StrT1)).Build(); err == nil {
		t.Fatal("accepted an ill-typed unary operand")
	}
}
