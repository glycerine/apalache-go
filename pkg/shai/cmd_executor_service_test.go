package shai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const remoteTrivialSpec = `---- MODULE M ----
Foo == TRUE
====`

const remoteCheckableSpec = `---- MODULE M ----
Init == TRUE
Next == TRUE
Inv == FALSE
====`

const remoteDeadlockSpec = `---- MODULE M ----
Init == TRUE
Next == FALSE
====`

const remoteMultiErrorSpec = `---- MODULE M ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2, 3}
Next == x' = x
Inv == x = 0
View == x
====`

const remoteIllTypedSpec = `---- MODULE M ----
\* @type: () => Int;
Foo == TRUE
====`

func remoteRequest(t *testing.T, command cmd.Cmd, source string, checker map[string]any) *cmd.CmdRequest {
	t.Helper()
	configuration := map[string]any{
		"source": map[string]any{"kind": "string", "content": source, "aux": []string{}, "format": "tla"},
	}
	if checker != nil {
		configuration["checker"] = checker
	}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	return &cmd.CmdRequest{Cmd: command, Config: string(encoded)}
}

func TestCmdExecutorParsesTypechecksAndFormatsTLA(t *testing.T) {
	service := NewCmdExecutorService()
	ctx := context.Background()
	if _, err := service.Ping(ctx, &cmd.PingRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, command := range []cmd.Cmd{cmd.Cmd_PARSE, cmd.Cmd_TYPECHECK} {
		response, err := service.Run(ctx, remoteRequest(t, command, remoteTrivialSpec, nil))
		if err != nil || response.GetSuccess() == "" {
			t.Fatalf("%s = %#v, %v", command, response, err)
		}
		var document map[string]any
		if err = json.Unmarshal([]byte(response.GetSuccess()), &document); err != nil || document["name"] != "ApalacheIR" {
			t.Fatalf("%s IR = %q, %v", command, response.GetSuccess(), err)
		}
	}

	response, err := service.Run(ctx, remoteRequest(t, cmd.Cmd_TLA, remoteTrivialSpec, nil))
	if err != nil || response.GetSuccess() == "" {
		t.Fatalf("TLA = %#v, %v", response, err)
	}
	var formatted string
	if err = json.Unmarshal([]byte(response.GetSuccess()), &formatted); err != nil {
		t.Fatal(err)
	}
	want := "----------------------------------- MODULE M -----------------------------------\n\n" +
		"EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants\n\n" +
		"Foo == TRUE\n\n" + strings.Repeat("=", 80) + "\n"
	if formatted != want {
		t.Fatalf("formatted TLA =\n%q\nwant\n%q", formatted, want)
	}
}

func TestCmdExecutorReportsFrontendAndConfigurationFailures(t *testing.T) {
	service := NewCmdExecutorService()
	ctx := context.Background()
	response, err := service.Run(ctx, remoteRequest(t, cmd.Cmd_PARSE, "INVALID SPEC", nil))
	if err != nil || response.GetFailure() == nil || !strings.Contains(strings.ToLower(response.GetFailure().GetData()), "module") {
		t.Fatalf("invalid parse = %q, %v", response.GetFailure().GetData(), err)
	}
	var parseFailure map[string]any
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &parseFailure); err != nil ||
		parseFailure["pass_name"] != "SanyParser" || parseFailure["exit_code"] != float64(255) {
		t.Fatalf("invalid parse shape = %#v, %v", parseFailure, err)
	}
	response, err = service.Run(ctx, &cmd.CmdRequest{Cmd: cmd.Cmd_PARSE, Config: `{}`})
	if err != nil || response.GetFailure() == nil || !strings.Contains(response.GetFailure().GetData(), "source") {
		t.Fatalf("missing source = %#v, %v", response, err)
	}
	response, err = service.Run(ctx, remoteRequest(t, cmd.Cmd_TYPECHECK, remoteIllTypedSpec, nil))
	if err != nil || response.GetFailure() == nil || response.GetFailure().GetErrorType() != cmd.CmdErrorType_PASS_FAILURE {
		t.Fatalf("ill-typed = %#v, %v", response, err)
	}
	var failure map[string]any
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil ||
		failure["pass_name"] != "TypeCheckerSnowcat" || failure["exit_code"] != float64(120) {
		t.Fatalf("ill-typed failure = %q, %v", response.GetFailure().GetData(), err)
	}

	response, err = service.Run(ctx, remoteRequest(t, cmd.Cmd_CHECK, remoteTrivialSpec, nil))
	if err != nil || response.GetFailure() == nil || !strings.Contains(response.GetFailure().GetData(), "Init") {
		t.Fatalf("invalid check = %#v, %v", response, err)
	}
}

func TestCmdExecutorChecksModelsAndReturnsCounterexamples(t *testing.T) {
	service := NewCmdExecutorService()
	ctx := context.Background()
	response, err := service.Run(ctx, remoteRequest(t, cmd.Cmd_CHECK, remoteCheckableSpec, nil))
	if err != nil || response.GetSuccess() == "" {
		t.Fatalf("valid check = %#v, %v", response, err)
	}
	response, err = service.Run(ctx, remoteRequest(t, cmd.Cmd_CHECK, remoteCheckableSpec, map[string]any{"inv": []string{"Inv"}}))
	if err != nil || response.GetFailure() == nil || response.GetFailure().GetErrorType() != cmd.CmdErrorType_PASS_FAILURE {
		t.Fatalf("violated invariant = %#v, %v", response, err)
	}
	var failure struct {
		PassName  string `json:"pass_name"`
		ExitCode  int    `json:"exit_code"`
		ErrorData struct {
			CheckingResult  string `json:"checking_result"`
			NErrors         int    `json:"nerrors"`
			Counterexamples []any  `json:"counterexamples"`
		} `json:"error_data"`
	}
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil ||
		failure.PassName != "BoundedChecker" || failure.ExitCode != 12 || failure.ErrorData.CheckingResult != "Error" ||
		failure.ErrorData.NErrors != 1 || len(failure.ErrorData.Counterexamples) == 0 {
		t.Fatalf("violation data = %+v, %v", failure, err)
	}
	response, err = service.Run(ctx, remoteRequest(t, cmd.Cmd_CHECK, remoteCheckableSpec, map[string]any{
		"inv": []string{"Inv"}, "length": 0, "no-deadlock": true, "discard-disabled": false,
		"tuning": map[string]string{
			"search.invariantFilter": "1->state0",
			"smt.randomSeed":         "17",
			"z3.smt.mbqi":            "true",
			"z3.sat.random_freq":     "0.25",
		},
	}))
	if err != nil || response.GetSuccess() == "" {
		t.Fatalf("filtered remote check = %#v, %v", response, err)
	}
}

func TestCmdExecutorReturnsMultipleDistinctCounterexamples(t *testing.T) {
	service := NewCmdExecutorService()
	response, err := service.Run(context.Background(), remoteRequest(t, cmd.Cmd_CHECK, remoteMultiErrorSpec, map[string]any{
		"inv": []string{"Inv"}, "view": "View", "max-error": 3, "length": 0, "no-deadlock": true,
	}))
	if err != nil || response.GetFailure() == nil {
		t.Fatalf("multi-error response = %#v, %v", response, err)
	}
	var failure struct {
		ExitCode  int `json:"exit_code"`
		ErrorData struct {
			CheckingResult  string `json:"checking_result"`
			NErrors         int    `json:"nerrors"`
			Counterexamples []any  `json:"counterexamples"`
		} `json:"error_data"`
	}
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil || failure.ExitCode != 12 ||
		failure.ErrorData.CheckingResult != "Error" || failure.ErrorData.NErrors != 3 ||
		len(failure.ErrorData.Counterexamples) != 3 {
		t.Fatalf("multi-error data = %+v, %v", failure, err)
	}
}

func TestCmdExecutorReturnsScalaDeadlockShape(t *testing.T) {
	service := NewCmdExecutorService()
	response, err := service.Run(context.Background(), remoteRequest(t, cmd.Cmd_CHECK, remoteDeadlockSpec,
		map[string]any{"length": 1}))
	if err != nil || response.GetFailure() == nil {
		t.Fatalf("deadlock response = %#v, %v", response, err)
	}
	var failure struct {
		PassName  string `json:"pass_name"`
		ExitCode  int    `json:"exit_code"`
		ErrorData struct {
			CheckingResult  string `json:"checking_result"`
			Counterexamples []any  `json:"counterexamples"`
			NErrors         *int   `json:"nerrors"`
		} `json:"error_data"`
	}
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil ||
		failure.PassName != "BoundedChecker" || failure.ExitCode != 12 ||
		failure.ErrorData.CheckingResult != "Deadlock" || len(failure.ErrorData.Counterexamples) != 1 ||
		failure.ErrorData.NErrors != nil {
		t.Fatalf("deadlock data = %+v, %v", failure, err)
	}
}

func TestCmdExecutorRejectsUnsafeAndMalformedRequests(t *testing.T) {
	service := NewCmdExecutorService()
	ctx := context.Background()
	for _, configuration := range []string{
		`{"source":"does-not-exist.tla"}`,
		`{"output":"result.json"}`,
		`{"checker":{"config":"M.cfg"}}`,
		`{"checker":{"tuning":{"z3.trace":"true"}}}`,
		`{"checker":{"tuning":{"search.outputTraces":"true"}}}`,
	} {
		if _, err := service.Run(ctx, &cmd.CmdRequest{Cmd: cmd.Cmd_PARSE, Config: configuration}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("unsafe config %s = %v", configuration, err)
		}
	}
	if _, err := service.Run(ctx, &cmd.CmdRequest{Cmd: cmd.Cmd(2), Config: `{}`}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown command = %v", err)
	}
	if _, err := service.Run(ctx, &cmd.CmdRequest{Cmd: cmd.Cmd_PARSE, Config: `{"unknown":true}`}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown config = %v", err)
	}
}

func TestRemoteConfigValidatorMatchesScalaFilesystemBoundary(t *testing.T) {
	unsafe := map[string]string{
		"$.config-file":    `{"config-file":"config.json"}`,
		"$.out-dir":        `{"out-dir":"out"}`,
		"$.run-dir":        `{"run-dir":"run"}`,
		"$.output":         `{"output":"output.tla"}`,
		"$.source":         `{"source":"input.tla"}`,
		"$.checker.config": `{"checker":{"config":"M.cfg"}}`,
		"$.tracee.trace":   `{"tracee":{"trace":"trace.itf.json"}}`,
	}
	for path, document := range unsafe {
		t.Run(path, func(t *testing.T) {
			if _, err := parseRemoteCommandConfig(document); err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("unsafe configuration error = %v, want path %s", err, path)
			}
		})
	}
	for key := range shaiFileWritingTuningKeys {
		document, err := json.Marshal(map[string]any{"checker": map[string]any{
			"tuning": map[string]string{key: "true"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = parseRemoteCommandConfig(string(document)); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("file-writing tuning %s error = %v", key, err)
		}
	}
}

func TestRemoteConfigValidatorAcceptsMemorySourceAndRejectsLegacyShape(t *testing.T) {
	document := `{"source":{"kind":"string","content":"---- MODULE M ----\n====","aux":[],"format":"tla"},` +
		`"checker":{"length":3,"tuning":{"search.invariant.mode":"before"}}}`
	config, err := parseRemoteCommandConfig(document)
	if err != nil || config.parsedSource == nil || config.parsedSource.content() != "---- MODULE M ----\n====" ||
		config.Checker.Length == nil || *config.Checker.Length != 3 {
		t.Fatalf("in-memory remote configuration = %#v, %v", config, err)
	}
	for _, legacy := range []struct {
		document, field string
	}{
		{`{"input":{"source":{"type":"string","content":"module M {}","format":"qnt"}}}`, "input"},
		{`{"checker":{"temporal-props":["q::temporalProps"]}}`, "temporal-props"},
	} {
		if _, err = parseRemoteCommandConfig(legacy.document); err == nil || !strings.Contains(err.Error(), legacy.field) {
			t.Fatalf("legacy field %s error = %v", legacy.field, err)
		}
	}
}
