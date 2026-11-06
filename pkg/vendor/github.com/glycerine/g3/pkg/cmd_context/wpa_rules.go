package cmd_context

// Source: wpa_parser_impl::finish_map_files and parse_rules_file.

import (
	"os"
)

func (p *wpaParser) readRules() error {
	files, err := p.files("rules")
	if err != nil {
		return err
	}
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// WPA registers domains once, then parses each rules file into the
		// same source constant tables that its relation tuples will use.
		p.rules.lexer = newDatalogLexer(contents)
		if err = p.rules.advance(); err != nil {
			return err
		}
		if err = p.rules.parseRules(); err != nil {
			return err
		}
	}
	return nil
}

func (p *wpaParser) indexDeclarations() error {
	engine, err := p.context.fixedpointContext()
	if err != nil {
		return err
	}
	declarations, err := engine.Predicates()
	if err != nil {
		return err
	}
	for _, declaration := range declarations {
		name, getErr := p.manager.GetASTName(declaration)
		if getErr != nil {
			return getErr
		}
		text, getErr := p.manager.Symbols.String(name)
		if getErr != nil {
			return getErr
		}
		p.declarations[text] = declaration
	}
	return nil
}
