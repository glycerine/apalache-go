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
	response, err = service.Run(ctx, &cmd.CmdRequest{Cmd: cmd.Cmd_PARSE, Config: `{}`})
	if err != nil || response.GetFailure() == nil || !strings.Contains(response.GetFailure().GetData(), "source") {
		t.Fatalf("missing source = %#v, %v", response, err)
	}
	response, err = service.Run(ctx, remoteRequest(t, cmd.Cmd_TYPECHECK, remoteIllTypedSpec, nil))
	if err != nil || response.GetFailure() == nil || response.GetFailure().GetErrorType() != cmd.CmdErrorType_PASS_FAILURE {
		t.Fatalf("ill-typed = %#v, %v", response, err)
	}
	var failure map[string]any
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil || failure["pass_name"] != "TypeCheckerSnowcat" {
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
		ErrorData struct {
			CheckingResult  string `json:"checking_result"`
			Counterexamples []any  `json:"counterexamples"`
		} `json:"error_data"`
	}
	if err = json.Unmarshal([]byte(response.GetFailure().GetData()), &failure); err != nil ||
		failure.PassName != "BoundedChecker" || failure.ErrorData.CheckingResult != "violation" || len(failure.ErrorData.Counterexamples) == 0 {
		t.Fatalf("violation data = %+v, %v", failure, err)
	}
	response, err = service.Run(ctx, remoteRequest(t, cmd.Cmd_CHECK, remoteCheckableSpec, map[string]any{
		"inv": []string{"Inv"}, "length": 0, "no-deadlock": true, "discard-disabled": false,
		"tuning": map[string]string{"search.invariantFilter": "1->state0", "smt.randomSeed": "17"},
	}))
	if err != nil || response.GetSuccess() == "" {
		t.Fatalf("filtered remote check = %#v, %v", response, err)
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
