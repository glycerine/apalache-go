package smt2

// Source: src/cmd_context/basic_cmds.cpp echo_cmd.

import "fmt"

func (p *commandParser) echoCommand() error {
	if p.token != StringToken {
		return fmt.Errorf("string expected")
	}
	value := p.scanner.String()
	if err := p.advance(); err != nil {
		return err
	}
	if err := p.requireEnd(); err != nil {
		return err
	}
	return p.context.WriteEcho(p.out, value)
}
