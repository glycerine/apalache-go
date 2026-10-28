package apalache

import "testing"

func TestTypeRewriterExpressionContracts(t *testing.T) {
	stringValue := NewValueExpr(StringValue("field"), Untyped())
	integer := NewValueExpr(Int64Value(1), Typed(BoolT1))
	name := NewNameExpr("x", Untyped())
	operation := MustOperExpr(Op("PLUS"), Untyped(), integer, name)
	local := NewOperDecl("Local", nil, stringValue, Untyped())
	let := NewLetInExpr(operation, Untyped(), local)
	types := map[UID]Type{
		integer.ID(): IntT1, name.ID(): IntT1, operation.ID(): IntT1,
		local.ID(): mustOperator(nil, StrT1), let.ID(): IntT1,
		// String values deliberately ignore inferred entries.
		stringValue.ID(): BoolT1,
	}
	rewriter := NewTypeRewriter(types, nil)
	rewritten, err := rewriter.Expression(let)
	if err != nil {
		t.Fatal(err)
	}
	result := rewritten.(*LetInExpr)
	if result == let || !EqualTypes(requireTagType(t, result.Tag()), IntT1) {
		t.Fatalf("rewritten LET = %v", result)
	}
	resultOperation := result.Body().(*OperExpr)
	if resultOperation == operation || !EqualTypes(requireTagType(t, resultOperation.Tag()), IntT1) {
		t.Fatal("operator was not freshly typed")
	}
	arguments := resultOperation.Arguments()
	if !EqualTypes(requireTagType(t, arguments[0].Tag()), IntT1) || !EqualTypes(requireTagType(t, arguments[1].Tag()), IntT1) {
		t.Fatalf("argument tags = %s, %s", arguments[0].Tag(), arguments[1].Tag())
	}
	resultLocal := result.Declarations()[0]
	if !EqualTypes(requireTagType(t, resultLocal.Tag()), mustOperator(nil, StrT1)) {
		t.Fatalf("local tag = %s", resultLocal.Tag())
	}
	writtenString := resultLocal.Body().(*ValueExpr)
	if !EqualTypes(requireTagType(t, writtenString.Tag()), StrT1) {
		t.Fatalf("string tag = %s, want model-value-or-string type", writtenString.Tag())
	}

	// Constructor ownership prevents later caller mutation from changing tags.
	types[name.ID()] = BoolT1
	again, err := rewriter.Expression(name)
	if err != nil || !EqualTypes(requireTagType(t, again.Tag()), IntT1) {
		t.Fatalf("rewriter retained caller map: %v, %v", again.Tag(), err)
	}
}

func TestTypeRewriterDefaultTagCallback(t *testing.T) {
	name := NewNameExpr("missing", Typed(IntT1))
	called := []UID{}
	rewriter := NewTypeRewriterWithDefault(nil, nil, func(id UID) TypeTag {
		called = append(called, id)
		return Typed(BoolT1)
	})
	rewritten, err := rewriter.Expression(name)
	if err != nil || len(called) != 1 || called[0] != name.ID() || !EqualTypes(requireTagType(t, rewritten.Tag()), BoolT1) {
		t.Fatalf("fallback result = %v, %v; calls = %v", rewritten, err, called)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("nil fallback accepted")
		}
	}()
	NewTypeRewriterWithDefault(nil, nil, nil)
}

func TestTypeRewriterDeclarationContracts(t *testing.T) {
	body := NewNameExpr("p", Untyped())
	types := map[UID]Type{body.ID(): BoolT1}
	constant := NewConstDecl("C", Untyped())
	variable := NewVarDecl("x", Untyped())
	variable.MarkInternal()
	assumption := NewNamedAssumeDecl("A", body, Untyped())
	unnamed := NewUnnamedAssumeDecl(body, Untyped())
	theorem := NewTheoremDecl("T", body, Untyped())
	operator := NewOperDecl("P", []OperatorParam{ValueParam("p")}, body, Untyped())
	operator.SetRecursive(true)
	for _, declaration := range []Decl{constant, variable, assumption, unnamed, theorem, operator} {
		types[declaration.ID()] = BoolT1
	}
	rewriter := NewTypeRewriter(types, nil)
	for _, declaration := range []Decl{constant, variable, assumption, unnamed, theorem, operator} {
		rewritten, err := rewriter.Declaration(declaration)
		if err != nil || rewritten == declaration || !EqualTypes(requireTagType(t, rewritten.Tag()), BoolT1) {
			t.Fatalf("rewrite %T = %v, %v", declaration, rewritten, err)
		}
		switch typed := rewritten.(type) {
		case *VarDecl:
			if !typed.IsInternal() {
				t.Fatal("internal variable marker was lost")
			}
		case *AssumeDecl:
			if name, named := declaration.(*AssumeDecl).DefinedName(); named {
				if got, ok := typed.DefinedName(); !ok || got != name {
					t.Fatal("named assumption identity was lost")
				}
			} else if _, named := typed.DefinedName(); named {
				t.Fatal("unnamed assumption acquired a name")
			}
		case *OperDecl:
			if !typed.IsRecursive() || len(typed.FormalParams()) != 1 {
				t.Fatal("operator metadata was lost")
			}
		}
	}
}

func TestTypeRewriterRejectsNullAndMalformedExcept(t *testing.T) {
	rewriter := NewTypeRewriter(nil, nil)
	if _, err := rewriter.Expression(nil); err == nil {
		t.Fatal("nil expression accepted")
	}
	if _, err := rewriter.Expression(NullExpr()); err == nil || err.Error() != "cannot apply TypeRewriter to NullExpr" {
		t.Fatalf("NullExpr error = %v", err)
	}
	if _, err := rewriter.Declaration(nil); err == nil {
		t.Fatal("nil declaration accepted")
	}
	bad := MustOperExpr(Op("EXCEPT"), Untyped(),
		NewNameExpr("f", Untyped()), NewNameExpr("not-a-tuple", Untyped()), NewValueExpr(Int64Value(1), Untyped()))
	if _, err := rewriter.Expression(bad); err == nil || err.Error() != "expected tuple accessor in EXCEPT" {
		t.Fatalf("malformed EXCEPT error = %v", err)
	}
}

func requireTagType(t *testing.T, tag TypeTag) Type {
	t.Helper()
	typ, ok := tag.Type()
	if !ok {
		t.Fatalf("tag is untyped: %s", tag)
	}
	return typ
}
