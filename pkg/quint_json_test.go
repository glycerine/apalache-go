package apalache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func setExpr(id string, arguments ...quintJSONExpr) quintJSONExpr {
	return quintJSONExpr{ID: json.Number(id), Kind: "app", Opcode: "Set", Args: arguments}
}

func listExpr(id string, arguments ...quintJSONExpr) quintJSONExpr {
	return quintJSONExpr{ID: json.Number(id), Kind: "app", Opcode: "List", Args: arguments}
}
