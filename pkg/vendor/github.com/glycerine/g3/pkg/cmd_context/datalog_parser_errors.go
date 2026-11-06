package cmd_context

import "fmt"

func (p *datalogParser) expected(what string) error {
	name := "unknown"
	if int(p.current.kind) < len(datalogTokenNames) {
		name = datalogTokenNames[p.current.kind]
	}
	return fmt.Errorf("datalog:%d:%d: expected %s, found %s %q", p.current.line, p.current.col, what, name, p.current.text)
}

func (p *datalogParser) require(kind datalogToken, what string) error {
	if p.current.kind != kind {
		return p.expected(what)
	}
	return nil
}
