package shai

import (
	"encoding/json"
	"fmt"
	"strings"

	pal "github.com/glycerine/apalache-go"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

type frontendPassFailure struct {
	PassName  string `json:"pass_name"`
	ErrorData any    `json:"error_data"`
	ExitCode  int    `json:"exit_code"`
}

func frontendFailure(result pal.FrontendResult) frontendPassFailure {
	// Parsed is assigned only after the parser has succeeded, before annotation
	// extraction and inference. Preserve that boundary even for CHECK requests.
	if result.Parsed == nil {
		// Scala's JSON/Quint reader returns a parser PassFailure directly;
		// SANY exceptions instead use the list-shaped generic-error path.
		for _, d := range result.Diagnostics {
			if d.Code == "E-IR-JSON" || (d.Code == "E-SPEC-PARSE" && strings.HasSuffix(d.Position.Filename, ".qnt.json")) {
				return frontendPassFailure{"SanyParser", d.Message, 150}
			}
		}
		messages := make([]string, 0, len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			if d.Severity == pal.SeverityError {
				messages = append(messages, d.Error())
			}
		}
		return frontendPassFailure{"SanyParser", messages, 255}
	}
	errors := make([][2]string, 0, len(result.Diagnostics))
	for _, d := range result.Diagnostics {
		if d.Severity != pal.SeverityError {
			continue
		}
		p := d.Position
		location := p.Filename
		if p.Line > 0 {
			location += fmt.Sprintf(":%d:%d", p.Line, p.Column)
		}
		if p.EndLine > 0 {
			location += fmt.Sprintf("-%d:%d", p.EndLine, p.EndColumn)
		}
		if location == "" {
			location = "<unknown>"
		}
		errors = append(errors, [2]string{location, d.Message})
	}
	return frontendPassFailure{"TypeCheckerSnowcat", errors, 120}
}

func frontendFailureCommandResponse(result pal.FrontendResult) *cmd.CmdResponse {
	data, _ := json.Marshal(frontendFailure(result))
	return &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{ErrorType: cmd.CmdErrorType_PASS_FAILURE, Data: string(data)}}}
}
