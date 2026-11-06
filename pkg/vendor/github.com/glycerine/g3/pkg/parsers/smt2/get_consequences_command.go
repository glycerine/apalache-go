package smt2

// Source: src/cmd_context/basic_cmds.cpp:810-845 get_consequences_cmd and
// src/cmd_context/cmd_context.cpp:1535-1557 cmd_context::get_consequences.

func (p *commandParser) getConsequencesCommand(execute bool) error {
	assumptions, err := p.expressions()
	if err != nil {
		return err
	}
	variables, err := p.expressions()
	if err != nil {
		return err
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	if !execute {
		return nil
	}
	return p.context.WriteConsequences(p.out, assumptions, variables)
}
