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
