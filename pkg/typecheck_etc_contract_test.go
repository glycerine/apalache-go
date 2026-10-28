package apalache

import "testing"

var _ TypeChecker = (*EtcTypeChecker)(nil)

func (listener *collectingTypeListener) errorExactly(id UID, message string) bool {
	for _, event := range listener.events {
		if event.kind == "error" && event.id == id && event.message == message {
			return true
		}
	}
	return false
}

func (listener *collectingTypeListener) count(kind string) int {
	count := 0
	for _, event := range listener.events {
		if event.kind == kind {
			count++
		}
	}
	return count
}

func TestEtcTypeCheckerApplicationContracts(t *testing.T) {
	tests := []struct {
		name       string
		signatures []string
		arguments  []string
		want       string
		wantError  string
	}{
		{name: "monomorphic", signatures: []string{"Int => Int"}, arguments: []string{"Int"}, want: "Int"},
		{name: "polymorphic", signatures: []string{"a => Seq(a)"}, arguments: []string{"Set(Int)"}, want: "Seq(Set(Int))"},
		{name: "unresolved argument", signatures: []string{"a => c"}, arguments: []string{"b"}, want: "c"},
		{name: "unresolved result", signatures: []string{"Int => a"}, arguments: []string{"Int"}, want: "a"},
		{name: "unique overload", signatures: []string{"Int => Int", "Bool => Bool"}, arguments: []string{"Int"}, want: "Int"},
		{name: "sound partial resolution", signatures: []string{"(a, Str) => Set(a)"}, arguments: []string{"Int", "b"}, want: "Set(Int)"},
		{
			name: "monomorphic mismatch", signatures: []string{"Int => Int"}, arguments: []string{"Bool"},
			wantError: "An operator with the signature ((Int) => Int) cannot be applied to the provided argument of type Bool",
		},
		{
			name: "one concrete and one unresolved overload", signatures: []string{"Int => a", "Int => Bool"}, arguments: []string{"Int"},
			wantError: "Annotation required. Found 2 matching operator signatures ((Int) => a) or ((Int) => Bool) for argument Int",
		},
		{
			name: "two generic overloads", signatures: []string{"a => Int", "a => Bool"}, arguments: []string{"Int"},
			wantError: "Annotation required. Found 2 matching operator signatures ((a) => Int) or ((a) => Bool) for argument Int",
		},
		{
			name: "partial unification is rejected", signatures: []string{"Seq(a) => Set(a)"}, arguments: []string{"a"},
			wantError: "An operator with the signature ((Seq(a)) => Set(a)) cannot be applied to the provided argument of type a",
		},
		{
			name: "tuple sequence ambiguity", signatures: []string{"(Int, Int) => Seq(Int)", "(Int, Int) => <<Int, Int>>"}, arguments: []string{"Int", "Int"},
			wantError: "Annotation required. Found 2 matching operator signatures ((Int, Int) => Seq(Int)) or ((Int, Int) => <<Int, Int>>) for arguments Int and Int",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signatures := make([]Type, len(test.signatures))
			for index, signature := range test.signatures {
				signatures[index] = parsedType(t, signature)
			}
			arguments := make([]EtcExpr, len(test.arguments))
			for index, argument := range test.arguments {
				arguments[index] = NewUniqueEtcConst(parsedType(t, argument))
			}
			application := NewEtcApp(ExactRef{SourceID: NewUID()}, signatures, arguments...)
			listener := &collectingTypeListener{}
			got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), application)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantError != "" {
				if ok || !listener.errorExactly(application.SourceRef().ID(), test.wantError) {
					t.Fatalf("Compute() = %v, %v; events = %+v", got, ok, listener.events)
				}
				return
			}
			if !ok || !EqualTypes(got, parsedType(t, test.want)) || !listener.found(application.SourceRef().ID(), parsedType(t, test.want)) {
				t.Fatalf("Compute() = %v, %v; events = %+v", got, ok, listener.events)
			}
		})
	}
}

func TestEtcTypeCheckerDoesNotCascadeNestedApplicationErrors(t *testing.T) {
	argument := NewUniqueEtcConst(BoolT1)
	inner := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "Int => Int")}, argument)
	outer := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "Int => Int")}, inner)
	listener := &collectingTypeListener{}
	if _, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), outer); err != nil || ok {
		t.Fatalf("Compute() = %v, %v", ok, err)
	}
	want := "An operator with the signature ((Int) => Int) cannot be applied to the provided argument of type Bool"
	if !listener.errorExactly(inner.SourceRef().ID(), want) ||
		!listener.errorExactly(outer.SourceRef().ID(), "Error when computing the type of the top expression") ||
		listener.count("error") != 2 {
		t.Fatalf("events = %+v", listener.events)
	}
}

func TestEtcTypeCheckerByNameContracts(t *testing.T) {
	a := variableType(t, "a")
	identity, _ := NewOperatorType([]Type{a}, a)
	scheme, _ := NewTypeScheme(identity, a.Number())
	context := NewTypeContext(map[string]TypeScheme{"F": scheme})

	intName, strName := NewUniqueEtcName("F"), NewUniqueEtcName("F")
	intCall := NewEtcAppByName(ExactRef{SourceID: NewUID()}, intName, NewUniqueEtcConst(IntT1))
	strCall := NewEtcAppByName(ExactRef{SourceID: NewUID()}, strName, NewUniqueEtcConst(StrT1))
	consume := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(Int, Str) => Bool")}, intCall, strCall)
	listener := &collectingTypeListener{}
	got, ok, err := newEtcChecker().Compute(listener, context, consume)
	if err != nil || !ok || !EqualTypes(got, BoolT1) ||
		!listener.found(intName.SourceRef().ID(), parsedType(t, "Int => Int")) ||
		!listener.found(strName.SourceRef().ID(), parsedType(t, "Str => Str")) {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}

	missing := NewEtcAppByName(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("Missing"))
	listener = &collectingTypeListener{}
	if _, ok, err = newEtcChecker().Compute(listener, EmptyTypeContext(), missing); err != nil || ok {
		t.Fatalf("missing Compute() = %v, %v", ok, err)
	}
	if !listener.errorExactly(missing.SourceRef().ID(), "The operator "+missing.Name().String()+" is used before it is defined.") {
		t.Fatalf("missing events = %+v", listener.events)
	}
}

func TestEtcTypeCheckerBinaryAbstractionContract(t *testing.T) {
	xName, yName := NewUniqueEtcName("x"), NewUniqueEtcName("y")
	xDomain := NewUniqueEtcConst(parsedType(t, "Set(Int)"))
	yDomain := NewUniqueEtcConst(parsedType(t, "Set(Str)"))
	body := NewUniqueEtcConst(BoolT1)
	abstraction := NewEtcAbs(ExactRef{SourceID: NewUID()}, body,
		EtcBinding{Name: xName, Domain: xDomain}, EtcBinding{Name: yName, Domain: yDomain})
	application := NewEtcApp(ExactRef{SourceID: NewUID()},
		[]Type{parsedType(t, "((a, b) => Bool) => Set(<<a, b>>)")}, abstraction)
	listener := &collectingTypeListener{}
	got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), application)
	if err != nil || !ok || !EqualTypes(got, parsedType(t, "Set(<<Int, Str>>)")) {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}
	checks := []struct {
		id  UID
		typ Type
	}{
		{xName.SourceRef().ID(), IntT1},
		{yName.SourceRef().ID(), StrT1},
		{xDomain.SourceRef().ID(), parsedType(t, "Set(Int)")},
		{yDomain.SourceRef().ID(), parsedType(t, "Set(Str)")},
		{abstraction.SourceRef().ID(), parsedType(t, "(Int, Str) => Bool")},
		{application.SourceRef().ID(), parsedType(t, "Set(<<Int, Str>>)")},
	}
	for _, check := range checks {
		if !listener.found(check.id, check.typ) {
			t.Errorf("missing type %s for %s in %+v", check.typ, check.id, listener.events)
		}
	}
}

func TestEtcTypeCheckerLetInferenceAndAnnotationContracts(t *testing.T) {
	for _, annotated := range []bool{false, true} {
		t.Run(map[bool]string{false: "inferred", true: "annotated"}[annotated], func(t *testing.T) {
			xDomain := NewUniqueEtcConst(parsedType(t, "Set(Int)"))
			xBinder := NewUniqueEtcName("x")
			xBody := NewUniqueEtcName("x")
			definition := NewEtcAbs(ExactRef{SourceID: NewUID()}, xBody, EtcBinding{Name: xBinder, Domain: xDomain})
			operatorName := NewUniqueEtcName("F")
			argument := NewUniqueEtcConst(IntT1)
			application := NewEtcAppByName(ExactRef{SourceID: NewUID()}, operatorName, argument)
			let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", definition, application)
			context := EmptyTypeContext()
			if annotated {
				scheme, _ := NewTypeScheme(parsedType(t, "Int => Int"))
				context = context.Bind("F", scheme)
			}
			listener := &collectingTypeListener{}
			got, ok, err := newEtcChecker().Compute(listener, context, let)
			if err != nil || !ok || !EqualTypes(got, IntT1) {
				t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
			}
			checks := []struct {
				id  UID
				typ Type
			}{
				{xDomain.SourceRef().ID(), parsedType(t, "Set(Int)")},
				{xBinder.SourceRef().ID(), IntT1},
				{xBody.SourceRef().ID(), IntT1},
				{definition.SourceRef().ID(), parsedType(t, "Int => Int")},
				{operatorName.SourceRef().ID(), parsedType(t, "Int => Int")},
				{argument.SourceRef().ID(), IntT1},
				{application.SourceRef().ID(), IntT1},
				{let.SourceRef().ID(), IntT1},
			}
			for _, check := range checks {
				if !listener.found(check.id, check.typ) {
					t.Errorf("missing type %s for %s in %+v", check.typ, check.id, listener.events)
				}
			}
		})
	}
}

func TestEtcTypeCheckerNullaryLetAndResultAnnotation(t *testing.T) {
	record := parsedType(t, "[foo: Int, bar: Str]")
	value := NewUniqueEtcConst(record)
	definition := NewEtcAbs(ExactRef{SourceID: NewUID()}, value)
	name := NewUniqueEtcName("F")
	application := NewEtcAppByName(ExactRef{SourceID: NewUID()}, name)
	let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", definition, application)
	// A result annotation is normalized to a nullary operator annotation.
	scheme, _ := NewTypeScheme(record)
	listener := &collectingTypeListener{}
	got, ok, err := newEtcChecker().Compute(listener, NewTypeContext(map[string]TypeScheme{"F": scheme}), let)
	if err != nil || !ok || !EqualTypes(got, record) {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}
	nullary := parsedType(t, "() => [foo: Int, bar: Str]")
	for _, check := range []struct {
		id  UID
		typ Type
	}{
		{value.SourceRef().ID(), record},
		{definition.SourceRef().ID(), nullary},
		{name.SourceRef().ID(), nullary},
		{application.SourceRef().ID(), record},
		{let.SourceRef().ID(), record},
	} {
		if !listener.found(check.id, check.typ) {
			t.Errorf("missing type %s for %s in %+v", check.typ, check.id, listener.events)
		}
	}
}

func TestEtcTypeCheckerWarnsAboutImpreciseAnnotation(t *testing.T) {
	x, y := NewUniqueEtcName("x"), NewUniqueEtcName("y")
	domainX := NewUniqueEtcConst(parsedType(t, "Set(f)"))
	domainY := NewUniqueEtcConst(parsedType(t, "Set(g)"))
	equality := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(c, c) => Bool")}, x, y)
	definition := NewEtcAbs(ExactRef{SourceID: NewUID()}, equality,
		EtcBinding{Name: x, Domain: domainX}, EtcBinding{Name: y, Domain: domainY})
	let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", definition, NewUniqueEtcConst(BoolT1))
	annotation := parsedType(t, "(a, b) => Bool")
	scheme, _ := NewTypeScheme(annotation, annotation.UsedNames()...)
	listener := &collectingTypeListener{}
	got, ok, err := newEtcChecker().Compute(listener, NewTypeContext(map[string]TypeScheme{"F": scheme}), let)
	if err != nil || !ok || !EqualTypes(got, BoolT1) {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}
	want := "F's type annotation ((a, b) => Bool) is too general, inferred: ((a, a) => Bool)"
	found := false
	for _, event := range listener.events {
		if event.kind == "warning" && event.id == let.SourceRef().ID() && event.message == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("events = %+v", listener.events)
	}
}

func TestEtcTypeCheckerRejectsAnnotatedParameterCountMismatch(t *testing.T) {
	x := NewUniqueEtcName("x")
	definition := NewEtcAbs(ExactRef{SourceID: NewUID()}, x,
		EtcBinding{Name: x, Domain: NewUniqueEtcConst(parsedType(t, "Set(Int)"))})
	let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", definition, NewUniqueEtcConst(BoolT1))
	scheme, _ := NewTypeScheme(parsedType(t, "(Int, Int) => Int"))
	listener := &collectingTypeListener{}
	if _, ok, err := newEtcChecker().Compute(listener, NewTypeContext(map[string]TypeScheme{"F": scheme}), let); err != nil || ok {
		t.Fatalf("Compute() = %v, %v", ok, err)
	}
	if !listener.errorExactly(definition.SourceRef().ID(), "Expected 2 parameters in F, found 1") {
		t.Fatalf("events = %+v", listener.events)
	}
}

func TestEtcTypeCheckerChooseAndStructuredApplicationContracts(t *testing.T) {
	t.Run("bounded choose", func(t *testing.T) {
		xRef, xBinder := NewUniqueEtcName("x"), NewUniqueEtcName("x")
		integer := NewUniqueEtcConst(IntT1)
		equality := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(a, a) => Bool")}, xRef, integer)
		domain := NewUniqueEtcConst(parsedType(t, "Set(Int)"))
		predicate := NewEtcAbs(ExactRef{SourceID: NewUID()}, equality, EtcBinding{Name: xBinder, Domain: domain})
		choose := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(a => Bool) => a")}, predicate)
		listener := &collectingTypeListener{}
		got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), choose)
		if err != nil || !ok || !EqualTypes(got, IntT1) {
			t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
		}
		for _, check := range []struct {
			id  UID
			typ Type
		}{
			{xRef.SourceRef().ID(), IntT1}, {xBinder.SourceRef().ID(), IntT1},
			{integer.SourceRef().ID(), IntT1}, {domain.SourceRef().ID(), parsedType(t, "Set(Int)")},
			{equality.SourceRef().ID(), BoolT1}, {predicate.SourceRef().ID(), parsedType(t, "Int => Bool")},
			{choose.SourceRef().ID(), IntT1},
		} {
			if !listener.found(check.id, check.typ) {
				t.Errorf("missing type %s for %s in %+v", check.typ, check.id, listener.events)
			}
		}
	})

	t.Run("unbounded choose propagates the predicate type", func(t *testing.T) {
		xRef, xBinder := NewUniqueEtcName("x"), NewUniqueEtcName("x")
		integer := NewUniqueEtcConst(IntT1)
		equality := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(a, a) => Bool")}, xRef, integer)
		domain := NewUniqueEtcConst(parsedType(t, "Set(b)"))
		predicate := NewEtcAbs(ExactRef{SourceID: NewUID()}, equality, EtcBinding{Name: xBinder, Domain: domain})
		choose := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(c => Bool) => c")}, predicate)
		listener := &collectingTypeListener{}
		got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), choose)
		if err != nil || !ok || !EqualTypes(got, IntT1) || !listener.found(choose.SourceRef().ID(), IntT1) {
			t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
		}
	})

	tests := []struct {
		name       string
		signatures []string
		arguments  []string
		want       string
	}{
		{
			name: "record set", signatures: []string{"(Set(a), Set(b)) => Set([x: a, y: b])"},
			arguments: []string{"Set(Int)", "Set(Str)"}, want: "Set([x: Int, y: Str])",
		},
		{
			name: "record constructor", signatures: []string{"(a, b) => [x: a, y: b]"},
			arguments: []string{"Str", "Int"}, want: "[x: Str, y: Int]",
		},
		{
			name: "domain of record", signatures: []string{
				"(a -> b) => Set(a)", "Seq(a) => Set(Int)", "[] => Set(Str)", "{} => Set(Int)",
			}, arguments: []string{"[foo: Int, bar: Bool]"}, want: "Set(Str)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signatures := make([]Type, len(test.signatures))
			for index, signature := range test.signatures {
				signatures[index] = parsedType(t, signature)
			}
			arguments := make([]EtcExpr, len(test.arguments))
			for index, argument := range test.arguments {
				arguments[index] = NewUniqueEtcConst(parsedType(t, argument))
			}
			application := NewEtcApp(ExactRef{SourceID: NewUID()}, signatures, arguments...)
			listener := &collectingTypeListener{}
			got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), application)
			if err != nil || !ok || !EqualTypes(got, parsedType(t, test.want)) ||
				!listener.found(application.SourceRef().ID(), parsedType(t, test.want)) {
				t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
			}
		})
	}
}

func TestEtcTypeCheckerRecursiveFunctionContract(t *testing.T) {
	recDomain := NewUniqueEtcConst(parsedType(t, "Set(c -> d)"))
	recBinder := NewUniqueEtcName("$recFun")
	xDomain := NewUniqueEtcConst(parsedType(t, "Set(Int)"))
	xBinder, xRef := NewUniqueEtcName("x"), NewUniqueEtcName("x")
	inner := NewEtcAbs(ExactRef{SourceID: NewUID()}, xRef, EtcBinding{Name: xBinder, Domain: xDomain})
	outer := NewEtcAbs(ExactRef{SourceID: NewUID()}, inner, EtcBinding{Name: recBinder, Domain: recDomain})
	application := NewEtcApp(ExactRef{SourceID: NewUID()},
		[]Type{parsedType(t, "((a -> b) => (a => b)) => a -> b")}, outer)
	listener := &collectingTypeListener{}
	got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), application)
	if err != nil || !ok || !EqualTypes(got, parsedType(t, "Int -> Int")) {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}
	for _, check := range []struct {
		id  UID
		typ Type
	}{
		{recDomain.SourceRef().ID(), parsedType(t, "Set(Int -> Int)")},
		{recBinder.SourceRef().ID(), parsedType(t, "Int -> Int")},
		{xDomain.SourceRef().ID(), parsedType(t, "Set(Int)")},
		{xBinder.SourceRef().ID(), IntT1}, {xRef.SourceRef().ID(), IntT1},
		{inner.SourceRef().ID(), parsedType(t, "Int => Int")},
		{outer.SourceRef().ID(), parsedType(t, "(Int -> Int) => (Int => Int)")},
		{application.SourceRef().ID(), parsedType(t, "Int -> Int")},
	} {
		if !listener.found(check.id, check.typ) {
			t.Errorf("missing type %s for %s in %+v", check.typ, check.id, listener.events)
		}
	}
}

func TestEtcTypeCheckerTypeDeclarationAndTupleAnnotationContracts(t *testing.T) {
	t.Run("type declaration", func(t *testing.T) {
		scoped := NewUniqueEtcName("foo")
		declaration := NewEtcTypeDecl(ExactRef{SourceID: NewUID()}, "foo", parsedType(t, "Set(Int)"), scoped)
		listener := &collectingTypeListener{}
		got, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), declaration)
		if err != nil || !ok || !EqualTypes(got, parsedType(t, "Set(Int)")) ||
			!listener.found(scoped.SourceRef().ID(), parsedType(t, "Set(Int)")) ||
			!listener.found(declaration.SourceRef().ID(), parsedType(t, "Set(Int)")) {
			t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
		}
	})

	t.Run("tuple annotation resolves overload", func(t *testing.T) {
		integer := NewUniqueEtcConst(IntT1)
		body := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{
			parsedType(t, "(Int, Int) => Seq(Int)"), parsedType(t, "(Int, Int) => <<Int, Int>>"),
		}, integer, integer)
		definition := NewEtcAbs(ExactRef{SourceID: NewUID()}, body)
		name := NewUniqueEtcName("F")
		application := NewEtcAppByName(ExactRef{SourceID: NewUID()}, name)
		let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", definition, application)
		tuple := parsedType(t, "<<Int, Int>>")
		scheme, _ := NewTypeScheme(tuple)
		listener := &collectingTypeListener{}
		got, ok, err := newEtcChecker().Compute(listener, NewTypeContext(map[string]TypeScheme{"F": scheme}), let)
		if err != nil || !ok || !EqualTypes(got, tuple) {
			t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
		}
		for _, id := range []UID{body.SourceRef().ID(), application.SourceRef().ID(), let.SourceRef().ID()} {
			if !listener.found(id, tuple) {
				t.Errorf("missing tuple type for %s in %+v", id, listener.events)
			}
		}
		if !listener.found(name.SourceRef().ID(), parsedType(t, "() => <<Int, Int>>")) {
			t.Errorf("missing nullary operator type in %+v", listener.events)
		}
	})
}

func TestEtcTypeCheckerPolymorphicLetContracts(t *testing.T) {
	for _, annotated := range []bool{false, true} {
		t.Run(map[bool]string{false: "inferred", true: "annotation variables do not override inference"}[annotated], func(t *testing.T) {
			domainVariable := "a"
			if annotated {
				domainVariable = "c"
			}
			xDomain := NewUniqueEtcConst(parsedType(t, "Set("+domainVariable+")"))
			xBinder, xRef := NewUniqueEtcName("x"), NewUniqueEtcName("x")
			d := NewUniqueEtcConst(parsedType(t, "d"))
			body := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(c, c) => c")}, xRef, d)
			definition := NewEtcAbs(ExactRef{SourceID: NewUID()}, body, EtcBinding{Name: xBinder, Domain: xDomain})
			name := NewUniqueEtcName("F")
			argument := NewUniqueEtcConst(IntT1)
			application := NewEtcAppByName(ExactRef{SourceID: NewUID()}, name, argument)
			let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", definition, application)
			context := EmptyTypeContext()
			if annotated {
				annotation := parsedType(t, "a => a")
				scheme, _ := NewTypeScheme(annotation, annotation.UsedNames()...)
				context = context.Bind("F", scheme)
			}
			listener := &collectingTypeListener{}
			got, ok, err := newEtcChecker().Compute(listener, context, let)
			if err != nil || !ok || !EqualTypes(got, IntT1) {
				t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
			}
			for _, check := range []struct {
				id  UID
				typ Type
			}{
				{xDomain.SourceRef().ID(), parsedType(t, "Set(a)")},
				{xBinder.SourceRef().ID(), parsedType(t, "a")}, {xRef.SourceRef().ID(), parsedType(t, "a")},
				{d.SourceRef().ID(), parsedType(t, "a")}, {body.SourceRef().ID(), parsedType(t, "a")},
				{definition.SourceRef().ID(), parsedType(t, "a => a")},
				{name.SourceRef().ID(), parsedType(t, "Int => Int")},
				{argument.SourceRef().ID(), IntT1}, {application.SourceRef().ID(), IntT1}, {let.SourceRef().ID(), IntT1},
			} {
				if !listener.found(check.id, check.typ) {
					t.Errorf("missing type %s for %s in %+v", check.typ, check.id, listener.events)
				}
			}
		})
	}
}

func TestEtcTypeCheckerPrincipalTypeRegression1259(t *testing.T) {
	f := NewUniqueEtcName("f")
	domainF := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(x -> y) => Set(x)")}, f)
	d := NewUniqueEtcName("d")
	callD := NewEtcAppByName(ExactRef{SourceID: NewUID()}, d)
	equality := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(z, z) => Bool")}, callD, domainF)
	f2 := NewUniqueEtcName("f2")
	callF2 := NewEtcAppByName(ExactRef{SourceID: NewUID()}, f2)
	domainF2 := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(i -> j) => Set(i)")}, callF2)
	definitionD := NewEtcAbs(ExactRef{SourceID: NewUID()}, domainF2)
	letD := NewEtcLet(ExactRef{SourceID: NewUID()}, "d", definitionD, equality)
	fAgain := NewUniqueEtcName("f")
	definitionF2 := NewEtcAbs(ExactRef{SourceID: NewUID()}, fAgain)
	letF2 := NewEtcLet(ExactRef{SourceID: NewUID()}, "f2", definitionF2, letD)
	fParameter := NewUniqueEtcConst(parsedType(t, "f"))
	fBinder := NewUniqueEtcName("f")
	definitionFoo := NewEtcAbs(ExactRef{SourceID: NewUID()}, letF2, EtcBinding{Name: fBinder, Domain: fParameter})
	letFoo := NewEtcLet(ExactRef{SourceID: NewUID()}, "Foo", definitionFoo, NewUniqueEtcConst(BoolT1))
	fooType := parsedType(t, "(a -> b) => Bool")
	scheme, _ := NewTypeScheme(fooType, fooType.UsedNames()...)
	listener := &collectingTypeListener{}
	got, ok, err := newEtcChecker().Compute(listener, NewTypeContext(map[string]TypeScheme{"Foo": scheme}), letFoo)
	if err != nil || !ok || !EqualTypes(got, BoolT1) || listener.count("error") != 0 {
		t.Fatalf("Compute() = %v, %v, %v; events = %+v", got, ok, err, listener.events)
	}
	if !listener.found(definitionD.SourceRef().ID(), parsedType(t, "() => Set(a)")) {
		t.Fatalf("principal type of d was not preserved: %+v", listener.events)
	}
}
