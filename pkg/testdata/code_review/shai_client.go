package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	trans "github.com/glycerine/apalache-go/shai/transexplorer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(os.Args[1], grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	c := cmd.NewCmdExecutorClient(conn)
	t := trans.NewTransExplorerClient(conn)
	out := map[string]any{}
	run := func(name string, command cmd.Cmd, config map[string]any) *cmd.CmdResponse {
		data, _ := json.Marshal(config)
		response, err := c.Run(ctx, &cmd.CmdRequest{Cmd: command, Config: string(data)})
		if err != nil {
			out[name] = err.Error()
		} else if response.GetFailure() != nil {
			out[name] = response.GetFailure()
		} else {
			out[name] = "success"
		}
		return response
	}
	src := "---- MODULE Audit ----\nEXTENDS Integers\nVARIABLE\n\\* @type: Int;\nx\nInit == x = 0\nNext == x' = x\n====\n"
	config := func(source, format string) map[string]any {
		return map[string]any{"source": map[string]any{"kind": "string", "content": source, "format": format}}
	}
	parsed := run("parse", cmd.Cmd_PARSE, config(src, "tla"))
	if parsed != nil && parsed.GetSuccess() != "" {
		run("json_parse", cmd.Cmd_PARSE, config(parsed.GetSuccess(), "json"))
	}
	for _, encoding := range []string{"arrays", "funArrays"} {
		cfg := config(src, "tla")
		cfg["checker"] = map[string]any{"smt-encoding": encoding, "length": 0}
		run(encoding, cmd.Cmd_CHECK, cfg)
	}
	rows := "---- MODULE RowDefault ----\nR == {[a |-> 1], [b |-> 2]}\n====\n"
	run("rows_default", cmd.Cmd_TYPECHECK, config(rows, "tla"))
	cfg := config(rows, "tla")
	cfg["features"] = []string{"rows"}
	run("rows_explicit", cmd.Cmd_TYPECHECK, cfg)
	for _, command := range []cmd.Cmd{cmd.Cmd_PARSE, cmd.Cmd_TYPECHECK, cmd.Cmd_CHECK} {
		run("syntax_"+command.String(), command, config("---- MODULE Bad ----\nO == (\n====\n", "tla"))
	}
	opened, err := t.OpenConnection(ctx, &trans.ConnectRequest{})
	if err != nil {
		panic(err)
	}
	loaded, err := t.LoadModel(ctx, &trans.LoadModelRequest{Conn: opened, Spec: "---- MODULE Bad ----\nO == (\n====\n"})
	if err != nil {
		out["load_bad"] = err.Error()
	} else {
		out["load_bad"] = loaded
	}
	upper := &trans.Connection{Id: strings.ToUpper(opened.Id)}
	loaded, err = t.LoadModel(ctx, &trans.LoadModelRequest{Conn: upper, Spec: src})
	if err != nil {
		out["uuid_upper"] = err.Error()
	} else if loaded.GetErr() != nil {
		out["uuid_upper"] = loaded.GetErr()
	} else {
		out["uuid_upper"] = "success"
	}
	encoded, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(encoded))
}
