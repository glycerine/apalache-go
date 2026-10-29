package apalache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
)

const quintBigIntFixture = `{
  "stage":"compiling",
  "modules":[{"id":17,"name":"t","declarations":[
    {"id":2,"kind":"var","name":"balance","typeAnnotation":{"kind":"int"},"depth":0},
    {"id":12,"kind":"def","name":"step","qualifier":"action","expr":{"id":11,"kind":"app","opcode":"assign","args":[{"id":10,"kind":"name","name":"balance"},{"id":9,"kind":"app","opcode":"iadd","args":[{"id":7,"kind":"name","name":"balance"},{"id":8,"kind":"int","value":10000}]}]}},
    {"id":16,"kind":"def","name":"inv","qualifier":"action","expr":{"id":15,"kind":"app","opcode":"ilt","args":[{"id":13,"kind":"name","name":"balance"},{"id":14,"kind":"int","value":10000}]}},
    {"id":6,"kind":"def","name":"init","qualifier":"action","expr":{"id":5,"kind":"app","opcode":"assign","args":[{"id":4,"kind":"name","name":"balance"},{"id":3,"kind":"int","value":100000000000}]}}
  ]}],
  "types":{
    "2":{"type":{"kind":"int"}},"3":{"type":{"kind":"int"}},"4":{"type":{"kind":"int"}},
    "5":{"type":{"kind":"bool"}},"6":{"type":{"kind":"bool"}},"7":{"type":{"kind":"int"}},
    "8":{"type":{"kind":"int"}},"9":{"type":{"kind":"int"}},"10":{"type":{"kind":"int"}},
    "11":{"type":{"kind":"bool"}},"12":{"type":{"kind":"bool"}},"13":{"type":{"kind":"int"}},
    "14":{"type":{"kind":"int"}},"15":{"type":{"kind":"bool"}},"16":{"type":{"kind":"bool"}}
  },"table":{}
}`

func TestParseQuintJSONPreservesTypedBigIntegers(t *testing.T) {
	module, err := ParseQuintJSON([]byte(quintBigIntFixture))
	if err != nil {
		t.Fatal(err)
	}
	if module.Name() != "t" || !module.HasProperty(PropertyTypeChecked) || len(module.Declarations()) != 4 {
		t.Fatalf("Quint module = %s, properties=%v, declarations=%d", module.Name(), module.HasProperty(PropertyTypeChecked), len(module.Declarations()))
	}
	init, found := declarationNamed(module, "init")
	if !found {
		t.Fatal("Quint init declaration is missing")
	}
	formatted, err := FormatTLAExpression(init.(*OperDecl).Body())
	if err != nil || !strings.Contains(formatted, "100000000000") {
		t.Fatalf("init expression = %s, %v", formatted, err)
	}
}

func TestQuintIRMatchesScalaBigIntAndAssumptionCases(t *testing.T) {
	large := "42" + "9223372036854775807" + "42"
	values := []string{"0", "1", "-1", large, "-" + large}
	importer := quintJSONImporter{types: map[string]quintJSONScheme{
		"1": {Type: quintJSONType{Kind: "int"}},
		"2": {Type: quintJSONType{Kind: "bool"}},
	}}
	for _, expected := range values {
		for _, raw := range []string{expected, fmt.Sprintf("%q", expected)} {
			expression := quintJSONExpr{ID: json.Number("1"), Kind: "int", Value: json.RawMessage(raw)}
			converted, err := importer.expression(expression)
			if err != nil {
				t.Fatalf("decode %s: %v", raw, err)
			}
			integer := converted.(*ValueExpr).Value().(IntValue).BigInt()
			want, _ := new(big.Int).SetString(expected, 10)
			if integer.Cmp(want) != 0 {
				t.Fatalf("decode %s = %s, want %s", raw, integer, want)
			}
			encoded, err := json.Marshal(expression)
			if err != nil || !strings.Contains(string(encoded), `"value":`+expected) {
				t.Fatalf("encode %s = %s, %v", raw, encoded, err)
			}
		}
	}

	truth := quintJSONExpr{ID: json.Number("2"), Kind: "bool", Value: json.RawMessage("true")}
	for _, test := range []struct {
		name  string
		named bool
	}{{"myAssume", true}, {"_", false}} {
		declaration, include, err := importer.declaration(quintJSONDef{
			ID: json.Number("3"), Kind: "assume", Name: test.name, Assumption: &truth,
		})
		if err != nil || !include {
			t.Fatalf("assumption %q = %#v, %v", test.name, declaration, err)
		}
		_, named := declaration.(*AssumeDecl).DefinedName()
		if named != test.named {
			t.Fatalf("assumption %q named = %v", test.name, named)
		}
	}

	if _, err := ParseQuintJSON([]byte(`{}`)); err == nil ||
		err.Error() != "input was not valid Quint JSON IR: missing keys in dictionary: stage, modules, types" {
		t.Fatalf("missing-key error = %v", err)
	}
}

func TestParseQuintJSONScalaOracleFixture(t *testing.T) {
	data, err := os.ReadFile("../json-rpc/src/test/resources/jsonrpc-bigint.qnt.json")
	if err != nil {
		t.Fatal(err)
	}
	module, err := ParseQuintJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if module.Name() != "t" || len(module.Declarations()) != 4 {
		t.Fatalf("oracle module = %s with %d declarations", module.Name(), len(module.Declarations()))
	}
}

func TestExplorationJSONRPCLoadsQuintJSON(t *testing.T) {
	handler := NewExplorationJSONRPCHandler()
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q],"format":"qnt","init":"init","next":"step"},"id":1}`,
		base64.StdEncoding.EncodeToString([]byte(quintBigIntFixture)))
	request := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body)).WithContext(context.Background())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		t.Fatalf("Quint load = %#v", envelope)
	}
	parameters, _ := result["specParameters"].(map[string]any)
	if len(parameters["initTransitions"].([]any)) != 1 || len(parameters["nextTransitions"].([]any)) != 1 {
		t.Fatalf("Quint parameters = %#v", parameters)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(quintBigIntFixture))
	body = fmt.Sprintf(`{"jsonrpc":"2.0","method":"loadSpec","params":{"sources":[%q,%q],"format":"qnt"},"id":2}`, encoded, encoded)
	request = httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	errorObject, _ := envelope["error"].(map[string]any)
	if errorObject["code"] != float64(JSONRPCInvalidParams) {
		t.Fatalf("auxiliary Quint input = %#v", envelope)
	}
}

func TestConvertQuintJSONTypeAlgebra(t *testing.T) {
	row := func(fields []quintJSONRecordField, other quintJSONRow) *quintJSONRow {
		return &quintJSONRow{Kind: "row", Fields: fields, Other: &other}
	}
	field := func(name, kind string) quintJSONRecordField {
		return quintJSONRecordField{FieldName: name, FieldType: quintJSONType{Kind: kind}}
	}
	empty := quintJSONRow{Kind: "empty"}
	open := quintJSONRow{Kind: "var", Name: "r"}
	a, _ := NewVarType(0)
	b, _ := NewVarType(1)
	closedRow, _ := NewRowType(map[string]Type{"f1": IntT1, "f2": StrT1}, nil)
	openRow, _ := NewRowType(map[string]Type{"f1": IntT1, "f2": StrT1}, &a)
	variantRow, _ := NewRowType(map[string]Type{"F1": IntT1, "F2": StrT1}, nil)
	unitRow, _ := NewRowType(map[string]Type{"tag": StrT1}, nil)
	sparse, _ := NewSparseTupleType(map[int]Type{1: IntT1, 2: StrT1})
	constant, _ := NewConstType("VALUE")

	tests := []struct {
		name   string
		source quintJSONType
		want   Type
	}{
		{"constant", quintJSONType{Kind: "const", Name: "VALUE"}, constant},
		{"variable", quintJSONType{Kind: "var", Name: "a"}, a},
		{"named variables", quintJSONType{Kind: "fun", Arg: &quintJSONType{Kind: "var", Name: "a"}, Res: &quintJSONType{Kind: "var", Name: "b"}}, mustFun(a, b)},
		{"repeated variable", quintJSONType{Kind: "fun", Arg: &quintJSONType{Kind: "var", Name: "a"}, Res: &quintJSONType{Kind: "var", Name: "a"}}, mustFun(a, a)},
		{"closed tuple", quintJSONType{Kind: "tup", Fields: row([]quintJSONRecordField{field("1", "str"), field("0", "int")}, empty)}, mustTuple(IntT1, StrT1)},
		{"open tuple", quintJSONType{Kind: "tup", Fields: row([]quintJSONRecordField{field("0", "int"), field("1", "str")}, open)}, sparse},
		{"empty tuple", quintJSONType{Kind: "tup", Fields: &empty}, NewRecordRowType(unitRow)},
		{"closed record", quintJSONType{Kind: "rec", Fields: row([]quintJSONRecordField{field("f1", "int"), field("f2", "str")}, empty)}, NewRecordRowType(closedRow)},
		{"open record", quintJSONType{Kind: "rec", Fields: row([]quintJSONRecordField{field("f1", "int"), field("f2", "str")}, open)}, NewRecordRowType(openRow)},
		{"sum", quintJSONType{Kind: "sum", Fields: row([]quintJSONRecordField{field("F1", "int"), field("F2", "str")}, empty)}, NewVariantType(variantRow)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := convertQuintJSONType(test.source)
			if err != nil {
				t.Fatal(err)
			}
			if !EqualTypes(got, test.want) {
				t.Fatalf("converted type = %s, want %s", got, test.want)
			}
		})
	}
}

func TestConvertAllQuintOracleTypes(t *testing.T) {
	for _, path := range []string{
		"../tla-io/src/test/resources/tictactoe.json",
		"../tla-io/src/test/resources/clockSync3.json",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(strings.NewReader(string(data)))
			decoder.UseNumber()
			var output quintJSONOutput
			if err = decoder.Decode(&output); err != nil {
				t.Fatal(err)
			}
			for id, scheme := range output.Types {
				if _, err = convertQuintJSONType(scheme.Type); err != nil {
					t.Fatalf("type %s: %v", id, err)
				}
			}
		})
	}
}

func TestConvertQuintCollectionExpressions(t *testing.T) {
	types := map[string]quintJSONScheme{
		"1": {Type: quintJSONType{Kind: "int"}},
		"2": {Type: quintJSONType{Kind: "int"}},
		"3": {Type: quintJSONType{Kind: "set", Elem: &quintJSONType{Kind: "int"}}},
		"4": {Type: quintJSONType{Kind: "bool"}},
		"5": {Type: quintJSONType{Kind: "list", Elem: &quintJSONType{Kind: "int"}}},
		"6": {Type: quintJSONType{Kind: "tup", Fields: &quintJSONRow{Kind: "empty"}}},
	}
	importer := quintJSONImporter{types: types}
	integer := func(id string, value int) quintJSONExpr {
		return quintJSONExpr{ID: json.Number(id), Kind: "int", Value: json.RawMessage(fmt.Sprintf("%d", value))}
	}
	application := func(id, opcode string, args ...quintJSONExpr) *OperExpr {
		expression, err := importer.expression(quintJSONExpr{ID: json.Number(id), Kind: "app", Opcode: opcode, Args: args})
		if err != nil {
			t.Fatalf("%s: %v", opcode, err)
		}
		operator, ok := expression.(*OperExpr)
		if !ok {
			t.Fatalf("%s converted to %T", opcode, expression)
		}
		return operator
	}

	set := application("3", "Set", integer("1", 1), integer("2", 2))
	if set.Operator().Name() != "SET_ENUM" {
		t.Fatalf("Set converted to %s", set.Operator().Name())
	}
	list := application("5", "List", integer("1", 1), integer("2", 2))
	if list.Operator().Name() != "TUPLE" {
		t.Fatalf("List converted to %s", list.Operator().Name())
	}
	unit := application("6", "Tup")
	if unit.Operator().Name() != "RECORD" || len(unit.Arguments()) != 2 {
		t.Fatalf("empty Tup converted to %s with %d arguments", unit.Operator().Name(), len(unit.Arguments()))
	}
	contains := application("4", "contains",
		quintJSONExpr{ID: json.Number("3"), Kind: "app", Opcode: "Set", Args: []quintJSONExpr{integer("1", 1)}},
		integer("2", 1))
	if contains.Operator().Name() != "SET_IN" || contains.Arguments()[0].(*ValueExpr).Value().(IntValue).BigInt().Int64() != 1 {
		t.Fatalf("contains converted to %v", contains)
	}
	for _, test := range []struct {
		opcode string
		want   string
		id     string
		args   []quintJSONExpr
	}{
		{"union", "SET_UNION2", "3", []quintJSONExpr{setExpr("3", integer("1", 1)), setExpr("3", integer("2", 2))}},
		{"powerset", "SET_POWERSET", "3", []quintJSONExpr{setExpr("3", integer("1", 1))}},
		{"append", "Sequences!Append", "5", []quintJSONExpr{listExpr("5", integer("1", 1)), integer("2", 2)}},
		{"length", "Sequences!Len", "1", []quintJSONExpr{listExpr("5", integer("1", 1))}},
		{"setOfMaps", "FUN_SET", "3", []quintJSONExpr{setExpr("3", integer("1", 1)), setExpr("3", integer("2", 2))}},
	} {
		if got := application(test.id, test.opcode, test.args...).Operator().Name(); got != test.want {
			t.Errorf("%s converted to %s, want %s", test.opcode, got, test.want)
		}
	}
}

func TestConvertQuintDerivedCollectionOperators(t *testing.T) {
	integer := quintJSONType{Kind: "int"}
	boolean := quintJSONType{Kind: "bool"}
	setOfInts := quintJSONType{Kind: "set", Elem: &integer}
	listOfInts := quintJSONType{Kind: "list", Elem: &integer}
	predicate := quintJSONType{Kind: "oper", Args: []quintJSONType{integer}, Res: &boolean}
	intMap := quintJSONType{Kind: "fun", Arg: &integer, Res: &integer}
	importer := quintJSONImporter{types: map[string]quintJSONScheme{
		"1": {Type: integer}, "2": {Type: integer}, "3": {Type: setOfInts},
		"4": {Type: boolean}, "5": {Type: listOfInts}, "7": {Type: predicate}, "8": {Type: intMap},
	}}
	integerExpr := func(id, value string) quintJSONExpr {
		return quintJSONExpr{ID: json.Number(id), Kind: "int", Value: json.RawMessage(value)}
	}
	nameExpr := func(id, name string) quintJSONExpr {
		return quintJSONExpr{ID: json.Number(id), Kind: "name", Name: name}
	}
	app := func(id, opcode string, args ...quintJSONExpr) Expr {
		result, err := importer.expression(quintJSONExpr{ID: json.Number(id), Kind: "app", Opcode: opcode, Args: args})
		if err != nil {
			t.Fatalf("%s: %v", opcode, err)
		}
		return result
	}
	set := setExpr("3", integerExpr("1", "1"), integerExpr("2", "2"))
	list := listExpr("5", integerExpr("1", "1"), integerExpr("2", "2"))

	tests := []struct {
		name     string
		expr     Expr
		operator string
	}{
		{"choose", app("1", "chooseSome", set), "CHOOSE3"},
		{"indices", app("3", "indices", list), "SET_MINUS"},
		{"range", app("5", "range", integerExpr("1", "2"), integerExpr("2", "7")), "Apalache!MkSeq"},
		{"select", app("5", "select", list, nameExpr("7", "Positive")), "Apalache!ApaFoldSeqLeft"},
		{"put", app("8", "put", nameExpr("8", "M"), integerExpr("1", "3"), integerExpr("2", "4")), "FUN_CTOR"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if countOperator(test.expr, test.operator) == 0 {
				t.Fatalf("converted expression does not contain %s: %v", test.operator, test.expr)
			}
			if result := (TypedLanguagePredicate{}).CheckExpression(test.expr); !result.OK() {
				t.Fatalf("converted expression is untyped: %+v", result.Failures)
			}
		})
	}
	if _, err := importer.expression(quintJSONExpr{ID: json.Number("1"), Kind: "app", Opcode: "oneOf", Args: []quintJSONExpr{set}}); err == nil {
		t.Fatal("standalone oneOf was accepted")
	}
}

func TestConvertQuintPredefinedNamesAndLocalLets(t *testing.T) {
	integer := quintJSONType{Kind: "int"}
	boolean := quintJSONType{Kind: "bool"}
	setInt := quintJSONType{Kind: "set", Elem: &integer}
	setBool := quintJSONType{Kind: "set", Elem: &boolean}
	importer := quintJSONImporter{types: map[string]quintJSONScheme{
		"1": {Type: integer}, "2": {Type: integer}, "3": {Type: integer},
		"4": {Type: integer}, "5": {Type: boolean}, "6": {Type: setInt}, "7": {Type: setBool},
	}}
	for _, test := range []struct {
		id   string
		name string
		want PredefinedSet
	}{
		{"7", "Bool", BoolSet}, {"6", "Int", IntSet}, {"6", "Nat", NatSet},
	} {
		expression, err := importer.expression(quintJSONExpr{ID: json.Number(test.id), Kind: "name", Name: test.name})
		if err != nil {
			t.Fatal(err)
		}
		if got := expression.(*ValueExpr).Value(); got != test.want {
			t.Errorf("%s converted to %v, want %v", test.name, got, test.want)
		}
	}

	localBody := quintJSONExpr{ID: json.Number("1"), Kind: "int", Value: json.RawMessage("7")}
	localName := quintJSONExpr{ID: json.Number("2"), Kind: "name", Name: "local"}
	local := quintJSONExpr{ID: json.Number("4"), Kind: "let",
		Opdef: &quintJSONDef{ID: json.Number("3"), Kind: "def", Name: "local", Qualifier: "val", Expr: &localBody},
		Expr:  &localName}
	converted, err := importer.expression(local)
	if err != nil {
		t.Fatal(err)
	}
	let, ok := converted.(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 || let.Declarations()[0].Name() != "local" {
		t.Fatalf("local let converted to %#v", converted)
	}
	application, ok := let.Body().(*OperExpr)
	if !ok || application.Operator().Name() != "OPER_APP" {
		t.Fatalf("local nullary reference converted to %v", let.Body())
	}

	truth := quintJSONExpr{ID: json.Number("5"), Kind: "bool", Value: json.RawMessage("true")}
	for _, labelName := range []string{"__label_Step", "main::__label_Step", "main::worker::__label_Step"} {
		label := quintJSONExpr{ID: json.Number("5"), Kind: "let",
			Opdef: &quintJSONDef{ID: json.Number("5"), Kind: "def", Name: labelName, Qualifier: "val", Expr: &truth},
			Expr:  &truth}
		converted, err = importer.expression(label)
		if err != nil {
			t.Fatal(err)
		}
		if countOperator(converted, "LABEL") != 1 {
			t.Fatalf("label let %q converted to %v", labelName, converted)
		}
	}

	generate := quintJSONExpr{ID: json.Number("6"), Kind: "app", Opcode: "apalache::generate",
		Args: []quintJSONExpr{{ID: json.Number("1"), Kind: "int", Value: json.RawMessage("5")}}}
	nondeterministic := quintJSONExpr{ID: json.Number("5"), Kind: "let",
		Opdef: &quintJSONDef{ID: json.Number("6"), Kind: "def", Name: "S", Qualifier: "nondet", Expr: &generate},
		Expr:  &truth}
	converted, err = importer.expression(nondeterministic)
	if err != nil {
		t.Fatal(err)
	}
	if countOperator(converted, "EXISTS3") != 1 || countOperator(converted, "Apalache!Gen") != 1 {
		t.Fatalf("nondet generate converted to %v", converted)
	}
}

func TestConvertQuintRejectsMalformedConstructors(t *testing.T) {
	integer := quintJSONType{Kind: "int"}
	stringType := quintJSONType{Kind: "str"}
	empty := quintJSONRow{Kind: "empty"}
	record := quintJSONType{Kind: "rec", Fields: &quintJSONRow{Kind: "row",
		Fields: []quintJSONRecordField{{FieldName: "x", FieldType: integer}}, Other: &empty}}
	variant := quintJSONType{Kind: "sum", Fields: &quintJSONRow{Kind: "row",
		Fields: []quintJSONRecordField{{FieldName: "Some", FieldType: integer}}, Other: &empty}}
	importer := quintJSONImporter{types: map[string]quintJSONScheme{
		"1": {Type: integer}, "2": {Type: stringType}, "3": {Type: record}, "4": {Type: variant},
	}}
	integerExpr := quintJSONExpr{ID: json.Number("1"), Kind: "int", Value: json.RawMessage("1")}
	stringExpr := quintJSONExpr{ID: json.Number("2"), Kind: "str", Value: json.RawMessage(`"Some"`)}
	tests := []quintJSONExpr{
		{ID: json.Number("3"), Kind: "app", Opcode: "Rec", Args: []quintJSONExpr{stringExpr}},
		{ID: json.Number("3"), Kind: "app", Opcode: "Rec", Args: []quintJSONExpr{integerExpr, integerExpr}},
		{ID: json.Number("4"), Kind: "app", Opcode: "variant", Args: []quintJSONExpr{integerExpr, integerExpr}},
		{ID: json.Number("1"), Kind: "app", Opcode: "variant", Args: []quintJSONExpr{stringExpr, integerExpr}},
	}
	for _, expression := range tests {
		if _, err := importer.expression(expression); err == nil {
			t.Errorf("malformed %s expression was accepted: %#v", expression.Opcode, expression)
		}
	}
}

func TestMarshalQuintTaggedUnionsMatchesScalaSchema(t *testing.T) {
	truth := quintJSONExpr{ID: json.Number("2"), Kind: "bool", Value: json.RawMessage("true"), Name: "ignored"}
	definition := quintJSONDef{
		ID: json.Number("1"), Kind: "def", Name: "myDef", Qualifier: "val", Expr: &truth,
		Doc: "ignored", Depth: json.Number("99"), TypeAnnotation: &quintJSONType{Kind: "str"},
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"def","id":1,"name":"myDef","qualifier":"val","expr":{"kind":"bool","id":2,"value":true}}`
	if string(encoded) != want {
		t.Fatalf("definition JSON = %s, want %s", encoded, want)
	}
	typeDefinition := quintJSONDef{ID: json.Number("3"), Kind: "typedef", Name: "myType"}
	encoded, err = json.Marshal(typeDefinition)
	if err != nil || string(encoded) != `{"kind":"typedef","id":3,"name":"myType"}` {
		t.Fatalf("type definition JSON = %s, %v", encoded, err)
	}
}

func TestMarshalQuintJSONIRCanonicalizesBigIntsAndTypeVariants(t *testing.T) {
	large := "42922337203685477580742"
	document := fmt.Sprintf(`{
		"stage":"typechecking","modules":[{"id":1,"name":"M","declarations":[
			{"kind":"def","id":2,"name":"n","qualifier":"val","expr":{"kind":"int","id":3,"value":%q}}
		]}],"types":{"3":{"type":{"id":99,"kind":"int"},"typeVariables":[],"rowVariables":[]}},
		"table":{},"warnings":[],"effects":{},"errors":[]}`, large)
	encoded, err := MarshalQuintJSONIR([]byte(document), false)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"value":`+large) || strings.Contains(text, `"value":"`+large+`"`) ||
		strings.Contains(text, `"id":99`) || strings.Contains(text, `"warnings"`) ||
		strings.Contains(text, `"effects"`) || strings.Contains(text, `"errors"`) {
		t.Fatalf("canonical Quint JSON = %s", text)
	}
	pretty, err := MarshalQuintJSONIR(encoded, true)
	if err != nil || !json.Valid(pretty) || !strings.Contains(string(pretty), "\n  \"stage\"") {
		t.Fatalf("pretty Quint JSON = %s, %v", pretty, err)
	}
}

func TestMarshalQuintJSONIRRoundTripsUpstreamFixtures(t *testing.T) {
	for _, path := range []string{
		"../tla-io/src/test/resources/tictactoe.json",
		"../tla-io/src/test/resources/clockSync3.json",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := MarshalQuintJSONIR(data, false)
			if err != nil {
				t.Fatal(err)
			}
			module, err := ParseQuintJSON(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if module.Name() == "" {
				t.Fatalf("round-tripped module = %#v", module)
			}
		})
	}
}

func TestMarshalQuintJSONIRRejectsMalformedAndTrailingInput(t *testing.T) {
	if _, err := MarshalQuintJSONIR([]byte(`{}`), false); err == nil || !strings.Contains(err.Error(), "missing keys in dictionary") {
		t.Errorf("empty document => %v", err)
	}
	for _, input := range []string{`{"foo":1}`, quintBigIntFixture + `{}`} {
		if _, err := MarshalQuintJSONIR([]byte(input), false); err == nil || !strings.Contains(err.Error(), "not valid Quint JSON IR") {
			t.Errorf("%q => %v", input[:min(len(input), 40)], err)
		}
	}
}

func TestConvertQuintSimpleOperatorSurface(t *testing.T) {
	importer := quintJSONImporter{types: map[string]quintJSONScheme{
		"1": {Type: quintJSONType{Kind: "int"}},
	}}
	opcodes := make([]string, 0, len(quintSimpleOperators))
	for opcode := range quintSimpleOperators {
		opcodes = append(opcodes, opcode)
	}
	sort.Strings(opcodes)
	for _, opcode := range opcodes {
		t.Run(opcode, func(t *testing.T) {
			operatorName := quintSimpleOperators[opcode]
			operator, found := LookupOperator(operatorName)
			if !found {
				t.Fatalf("mapped operator %q is not registered", operatorName)
			}
			arity := -1
			for count := 0; count <= 8; count++ {
				if operator.IsCorrectArity(count) {
					arity = count
					break
				}
			}
			if arity < 0 {
				t.Fatalf("could not find a small valid arity for %q", operatorName)
			}
			arguments := make([]quintJSONExpr, arity)
			for index := range arguments {
				arguments[index] = quintJSONExpr{ID: json.Number("1"), Kind: "int", Value: json.RawMessage("1")}
			}
			converted, err := importer.expression(quintJSONExpr{
				ID: json.Number("1"), Kind: "app", Opcode: opcode, Args: arguments,
			})
			if err != nil {
				t.Fatal(err)
			}
			application, ok := converted.(*OperExpr)
			if !ok || application.Operator().Name() != operatorName {
				t.Fatalf("%s converted to %v", opcode, converted)
			}
		})
	}
}

func setExpr(id string, arguments ...quintJSONExpr) quintJSONExpr {
	return quintJSONExpr{ID: json.Number(id), Kind: "app", Opcode: "Set", Args: arguments}
}

func listExpr(id string, arguments ...quintJSONExpr) quintJSONExpr {
	return quintJSONExpr{ID: json.Number(id), Kind: "app", Opcode: "List", Args: arguments}
}

func TestConvertQuintParameterizedDefinitionAndCall(t *testing.T) {
	integer := quintJSONType{Kind: "int"}
	operator := quintJSONType{Kind: "oper", Args: []quintJSONType{integer}, Res: &integer}
	importer := quintJSONImporter{types: map[string]quintJSONScheme{
		"1": {Type: integer},
		"2": {Type: integer},
		"3": {Type: integer},
		"4": {Type: operator},
	}}
	body := quintJSONExpr{ID: json.Number("3"), Kind: "app", Opcode: "iadd", Args: []quintJSONExpr{
		{ID: json.Number("1"), Kind: "name", Name: "x"},
		{ID: json.Number("2"), Kind: "int", Value: json.RawMessage("1")},
	}}
	lambda := quintJSONExpr{ID: json.Number("4"), Kind: "lambda",
		Params: []quintJSONLambdaParam{{ID: json.Number("1"), Name: "x"}}, Expr: &body}
	declaration, include, err := importer.declaration(quintJSONDef{
		ID: json.Number("4"), Kind: "def", Name: "Inc", Qualifier: "def", Expr: &lambda,
	})
	if err != nil {
		t.Fatal(err)
	}
	defined := declaration.(*OperDecl)
	if !include || len(defined.FormalParams()) != 1 || defined.FormalParams()[0] != ValueParam("x") || defined.Body().(*OperExpr).Operator().Name() != "PLUS" {
		t.Fatalf("parameterized declaration = %#v", defined)
	}

	call, err := importer.expression(quintJSONExpr{ID: json.Number("3"), Kind: "app", Opcode: "Inc",
		Args: []quintJSONExpr{{ID: json.Number("2"), Kind: "int", Value: json.RawMessage("41")}}})
	if err != nil {
		t.Fatal(err)
	}
	application := call.(*OperExpr)
	callee, ok := application.Arguments()[0].(*NameExpr)
	if application.Operator().Name() != "OPER_APP" || !ok || callee.Name() != "Inc" {
		t.Fatalf("defined application = %v", application)
	}
}

func TestParseQuintTicTacToeOracle(t *testing.T) {
	data, err := os.ReadFile("../tla-io/src/test/resources/tictactoe.json")
	if err != nil {
		t.Fatal(err)
	}
	module, err := ParseQuintJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Declarations()) != 40 || !module.HasProperty(PropertyTypeChecked) {
		t.Fatalf("parsed module has %d declarations and typechecked=%v", len(module.Declarations()), module.HasProperty(PropertyTypeChecked))
	}
	if result := (TypedLanguagePredicate{}).CheckModule(module); !result.OK() {
		t.Fatalf("parsed module contains untyped IR: %+v", result.Failures)
	}
}

func TestExplorationLoadsQuintTicTacToeOracle(t *testing.T) {
	data, err := os.ReadFile("../tla-io/src/test/resources/tictactoe.json")
	if err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(loadSpecParams{
		Sources: []string{base64.StdEncoding.EncodeToString(data)},
		Init:    "init", Next: "step", Invariants: []string{"inv"}, Format: "qnt",
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewExplorationService()
	result, rpcErr := service.loadSpec(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("loadSpec failed: %+v", rpcErr)
	}
	loaded := result.(loadSpecResult)
	if len(loaded.SpecParameters.InitTransitions) == 0 || len(loaded.SpecParameters.NextTransitions) == 0 ||
		len(loaded.SpecParameters.StateInvariants) != 1 {
		t.Fatalf("loaded parameters = %#v", loaded.SpecParameters)
	}
	assume, err := json.Marshal(assumeTransitionParams{
		SessionID: loaded.SessionID, TransitionID: 0, CheckEnabled: true, TimeoutSec: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	assumed, rpcErr := service.assumeTransition(context.Background(), assume)
	if rpcErr != nil {
		t.Fatalf("assumeTransition failed: %+v", rpcErr)
	}
	status := assumed.(assumeTransitionResult).Status
	if status != "ENABLED" {
		t.Fatalf("initial transition status = %q, want ENABLED", status)
	}
}
