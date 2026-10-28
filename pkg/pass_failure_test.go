package apalache

import (
	"encoding/json"
	"testing"
)

func TestPassFailureMatchesScalaJSONShape(t *testing.T) {
	failure, err := NewPassFailure("TestPass", map[string]any{"reason": "unsupported", "line": 4}, ExitCodeSpecEvaluation)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		PassName  string         `json:"pass_name"`
		ErrorData map[string]any `json:"error_data"`
		ExitCode  int            `json:"exit_code"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.PassName != "TestPass" || decoded.ErrorData["reason"] != "unsupported" || decoded.ErrorData["line"] != float64(4) || decoded.ExitCode != 75 {
		t.Fatalf("pass failure JSON = %s", encoded)
	}
}

func TestPassFailureEncodesAbsentDataAsNull(t *testing.T) {
	failure, err := NewPassFailure("TestPass", nil, ExitCodeError)
	if err != nil {
		t.Fatal(err)
	}
	if string(failure.ErrorData) != "null" || failure.Error() != "pass TestPass failed with exit code 255" {
		t.Fatalf("nil-data pass failure = %#v, %q", failure, failure.Error())
	}
}
