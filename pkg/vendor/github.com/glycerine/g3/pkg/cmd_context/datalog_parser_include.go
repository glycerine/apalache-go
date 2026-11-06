package cmd_context

// Source: dparser::parse_include with source-relative nested resolution.

import (
	"fmt"
	"os"
	"path/filepath"
)

func (p *datalogParser) parseInclude(domains bool) error {
	if err := p.advance(); err != nil {
		return err
	}
	if p.current.kind != dlString {
		return p.expected("include filename string")
	}
	filename := p.current.text
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(p.path, filename)
	}
	input, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("datalog include %s: %w", filename, err)
	}
	outerLexer, outerCurrent, outerPath := p.lexer, p.current, p.path
	p.lexer, p.path = newDatalogLexer(input), filepath.Dir(filename)
	p.lexer.parsingDomains = domains
	if err = p.advance(); err == nil {
		if domains {
			err = p.parseDomains()
		} else {
			err = p.parseRules()
		}
	}
	p.lexer, p.current, p.path = outerLexer, outerCurrent, outerPath
	if err != nil {
		return err
	}
	if err = p.advance(); err != nil {
		return err
	}
	if domains && p.current.kind == dlNewline {
		return p.advance()
	}
	return nil
}
