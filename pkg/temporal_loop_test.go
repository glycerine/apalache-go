package apalache

import (
	"fmt"
	"testing"
)

func temporalTestModule() *Module {
	x := NewVarDecl("x", Typed(IntT1))
	flag := NewVarDecl("flag", Typed(BoolT1))
	init := boolDefinition("Init", MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), temporalName(x), intLiteral(0)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), temporalName(flag), temporalBool(false))))
	next := boolDefinition("Next", MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), temporalPrime(temporalName(x)), temporalName(x)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), temporalPrime(temporalName(flag)), temporalName(flag))))
	module := NewModule("Temporal", []Decl{x, flag, init, next})
	module.AddProperty(PropertyTypeChecked)
	return module
}

func TestTemporalLoopEncoderAddsTypedAuxiliariesAndPredicates(t *testing.T) {
	module := temporalTestModule()
	result, err := NewTemporalLoopEncoder(nil).AddLoopLogic(module, "Init", "Next")
	if err != nil {
		t.Fatal(err)
	}
	variables := result.Module.VarDeclarations()
	wantNames := []string{"__saved_x", "__saved_flag", "__InLoop", "x", "flag"}
	if len(variables) != len(wantNames) {
		t.Fatalf("variables = %v", declarationNameSlice(variables))
	}
	for index, want := range wantNames {
		if variables[index].Name() != want {
			t.Fatalf("variable %d = %s, want %s", index, variables[index].Name(), want)
		}
	}
	if result.LoopOK.Name() != TemporalLoopOKName ||
		countOperator(result.Init.Body(), "EQ") != 5 || countOperator(result.Next.Body(), "IF_THEN_ELSE") != 1 {
		t.Fatalf("loop encoding init=%v next=%v loop=%v", result.Init.Body(), result.Next.Body(), result.LoopOK.Body())
	}
	if !result.Module.HasProperty(PropertyTypeChecked) {
		t.Fatal("module properties were not preserved")
	}
}

func TestTemporalLoopEncoderScalaGeneratedModuleContracts(t *testing.T) {
	types := []Type{IntT1, BoolT1, StrT1}
	for sample := 0; sample < 500; sample++ {
		variableCount := sample % 9
		declarations := make([]Decl, 0, variableCount+2)
		originals := make([]*VarDecl, variableCount)
		for index := range variableCount {
			variable := NewVarDecl(fmt.Sprintf("v%d_%d", sample, index), Typed(types[(sample+index)%len(types)]))
			originals[index] = variable
			declarations = append(declarations, variable)
		}
		declarations = append(declarations,
			boolDefinition("Init", temporalBool(true)),
			boolDefinition("Next", temporalBool(true)))

		result, err := NewTemporalLoopEncoder(nil).AddLoopLogic(NewModule("Generated", declarations), "Init", "Next")
		if err != nil {
			t.Fatalf("sample %d: %v", sample, err)
		}

		variables := result.Module.VarDeclarations()
		if len(variables) != 2*variableCount+1 {
			t.Fatalf("sample %d: variables = %v", sample, declarationNameSlice(variables))
		}
		nameCounts := make(map[string]int, len(variables))
		for _, variable := range variables {
			nameCounts[variable.Name()]++
		}
		for _, original := range originals {
			savedName := TemporalSavedPrefix + original.Name()
			if nameCounts[original.Name()] != 1 || nameCounts[savedName] != 1 {
				t.Fatalf("sample %d: missing original/copy pair for %s in %v", sample, original.Name(), nameCounts)
			}
			var saved *VarDecl
			for _, variable := range variables {
				if variable.Name() == savedName {
					saved = variable
					break
				}
			}
			if saved == nil || !saved.Tag().Equal(original.Tag()) || !saved.IsInternal() {
				t.Fatalf("sample %d: invalid loop copy for %s: %#v", sample, original.Name(), saved)
			}
		}
		if nameCounts[TemporalInLoopName] != 1 || nameCounts[TemporalSavedPrefix+TemporalInLoopName] != 0 {
			t.Fatalf("sample %d: invalid InLoop declarations: %v", sample, nameCounts)
		}
		loopOKCount := 0
		for _, declaration := range result.Module.OperDeclarations() {
			if declaration.Name() == TemporalLoopOKName {
				loopOKCount++
			}
		}
		if loopOKCount != 1 {
			t.Fatalf("sample %d: LoopOK declaration count = %d", sample, loopOKCount)
		}
	}
}

func TestTemporalLoopEncoderSavedUpdateMatchesScalaRule(t *testing.T) {
	variable := NewVarDecl("x", Typed(IntT1))
	saved := NewVarDecl("__saved_x", Typed(IntT1))
	update := NewTemporalLoopEncoder(nil).SavedVariableUpdate(variable, saved)
	equality := update.(*OperExpr)
	if equality.Operator().Name() != "EQ" || equality.Arguments()[0].(*OperExpr).Operator().Name() != "PRIME" {
		t.Fatalf("saved update = %v", update)
	}
	ite := equality.Arguments()[1].(*OperExpr)
	if ite.Operator().Name() != "IF_THEN_ELSE" || ite.Arguments()[1].(*NameExpr).Name() != "__saved_x" ||
		ite.Arguments()[2].(*NameExpr).Name() != "x" {
		t.Fatalf("saved update value = %v", ite)
	}
}

func TestTemporalLoopEncoderHelpersUseScalaZipSemantics(t *testing.T) {
	encoder := NewTemporalLoopEncoder(nil)
	x := NewVarDecl("x", Typed(IntT1))
	y := NewVarDecl("y", Typed(IntT1))
	savedX := NewVarDecl("__saved_x", Typed(IntT1))
	init := boolDefinition("Init", temporalBool(true))
	next := boolDefinition("Next", temporalBool(true))

	if got := countOperator(encoder.AddToInit([]*VarDecl{x, y}, []*VarDecl{savedX}, init).Body(), "EQ"); got != 2 {
		t.Fatalf("init equality count = %d, want InLoop plus one zipped pair", got)
	}
	encodedNext := encoder.AddToNext([]*VarDecl{x, y}, []*VarDecl{savedX}, next)
	if got := countNamed(encodedNext.Body(), "y"); got != 0 {
		t.Fatalf("unpaired variable occurred %d times in next encoding", got)
	}
	if got := countOperator(encoder.LoopPredicate([]*VarDecl{x, y}, []*VarDecl{savedX}).Body(), "EQ"); got != 1 {
		t.Fatalf("loop predicate equality count = %d, want one zipped pair", got)
	}

	empty := NewModule("Empty", []Decl{init, next})
	result, err := encoder.AddLoopLogic(empty, "Init", "Next")
	if err != nil || len(result.Module.VarDeclarations()) != 1 || result.Module.VarDeclarations()[0].Name() != TemporalInLoopName {
		t.Fatalf("empty-state loop encoding = %#v, %v", result, err)
	}
}

func TestTemporalLoopEncoderRejectsMissingAndUntypedInputs(t *testing.T) {
	module := temporalTestModule()
	if _, err := NewTemporalLoopEncoder(nil).AddLoopLogic(module, "Missing", "Next"); err == nil {
		t.Fatal("missing init was accepted")
	}
	untyped := NewModule("Bad", []Decl{
		NewVarDecl("x", Untyped()),
		boolDefinition("Init", temporalBool(true)), boolDefinition("Next", temporalBool(true)),
	})
	if _, err := NewTemporalLoopEncoder(nil).AddLoopLogic(untyped, "Init", "Next"); err == nil {
		t.Fatal("untyped variable was accepted")
	}
}

func TestTemporalUtilitiesAppendAndFlattenDeclaration(t *testing.T) {
	leftInit := temporalBool(true)
	rightInit := temporalBool(false)
	left := TemporalPredicateExpressions{Init: []Expr{leftInit}, Next: []Expr{temporalBool(true)}}
	right := TemporalPredicateExpressions{Init: []Expr{rightInit}, LoopOK: []Expr{temporalBool(false)}}
	combined := left.Append(right)
	if len(combined.Init) != 2 || combined.Init[0] != leftInit || combined.Init[1] != rightInit ||
		len(combined.Next) != 1 || len(combined.LoopOK) != 1 {
		t.Fatalf("appended temporal predicates = %#v", combined)
	}
	combined.Init[0] = rightInit
	if left.Init[0] != leftInit {
		t.Fatal("temporal predicate append mutated its input")
	}

	p := NewNameExpr("p", Typed(BoolT1))
	q := NewNameExpr("q", Typed(BoolT1))
	r := NewNameExpr("r", Typed(BoolT1))
	declaration := boolDefinition("Pred", temporalAnd(p, q))
	extended := NewTemporalLoopEncoder(nil).extendDeclaration(declaration, r)
	body, ok := extended.Body().(*OperExpr)
	if !ok || body.Operator().Name() != "AND" || len(body.Arguments()) != 3 {
		t.Fatalf("extended declaration was not flattened: %v", extended.Body())
	}
	if declaration == extended || len(declaration.Body().(*OperExpr).Arguments()) != 2 {
		t.Fatal("extending a declaration mutated or reused the input")
	}
}

func TestEncodeTemporalPropertiesChecksDependencyAndMarksNoOp(t *testing.T) {
	module := NewModule("M", nil)
	if _, _, err := EncodeTemporalProperties(module, DerivedPredicates{}, nil, nil, nil); err == nil {
		t.Fatal("temporal encoding accepted a module without the TypeChecked property")
	}
	module.AddProperty(PropertyTypeChecked)
	encoded, predicates, err := EncodeTemporalProperties(module, DerivedPredicates{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !encoded.HasProperty(PropertyTemporalEncoded) || !encoded.HasProperty(PropertyTypeChecked) {
		t.Fatal("no-op temporal pass did not preserve and add properties")
	}
	if module.HasProperty(PropertyTemporalEncoded) || encoded == module || len(predicates.TemporalProperties) != 0 {
		t.Fatal("no-op temporal pass mutated or reused its input")
	}
}

func TestTemporalLoopEncodingFeedsProductionPreprocessor(t *testing.T) {
	result, err := NewTemporalLoopEncoder(nil).AddLoopLogic(temporalTestModule(), "Init", "Next")
	if err != nil {
		t.Fatal(err)
	}
	result.Module.AddProperty(PropertyInlined)
	preprocessed, err := PreprocessModule(result.Module, PreprocessOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if countOperator(preprocessed.OperDeclarations()[1].Body(), "UNCHANGED") != 0 {
		t.Fatal("temporal loop update retained UNCHANGED after preprocessing")
	}
	if !preprocessed.HasProperty(PropertyPreprocessed) {
		t.Fatal("temporal loop module was not marked preprocessed")
	}
}
