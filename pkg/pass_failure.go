package apalache

import (
	"encoding/json"
	"fmt"
)

// PassFailure is the structured failure returned by a module pass. Its JSON
// representation matches Scala's Pass.PassFailure wire format.
type PassFailure struct {
	PassName  string          `json:"pass_name"`
	ErrorData json.RawMessage `json:"error_data"`
	ExitCode  ExitCode        `json:"exit_code"`
}

func NewPassFailure(passName string, errorData any, exitCode ExitCode) (*PassFailure, error) {
	encoded, err := json.Marshal(errorData)
	if err != nil {
		return nil, fmt.Errorf("encode failure data for pass %s: %w", passName, err)
	}
	return &PassFailure{PassName: passName, ErrorData: encoded, ExitCode: exitCode}, nil
}

func (failure *PassFailure) Error() string {
	if failure == nil {
		return "<nil pass failure>"
	}
	return fmt.Sprintf("pass %s failed with exit code %d", failure.PassName, failure.ExitCode)
}
