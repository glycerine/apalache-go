package cmd_context

// Source: src/muz/fp/datalog_parser.cpp dparser retained API/shell parser.

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/glycerine/g3/pkg/ast"
)

type datalogParser struct {
	context    *Context
	manager    *ast.ASTManager
	lexer      *datalogLexer
	current    datalogLexeme
	path       string
	sorts      map[string]ast.ASTHandle
	predicates map[string]ast.ASTHandle
	constants  map[ast.ASTHandle]map[string]uint64
	// WPA numeric domains use uint64 keys for relation tuples. Ordinary
	// Datalog finite domains keep their existing symbolic constant identity.
	numericConstants map[ast.ASTHandle]map[uint64]uint64
	variables        map[string]ast.ASTHandle
	nextVar          uint32
	pins             []ast.ASTRef
}

func newDatalogParser(context *Context) (*datalogParser, error) {
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	return &datalogParser{context: context, manager: manager, sorts: make(map[string]ast.ASTHandle), predicates: make(map[string]ast.ASTHandle), constants: make(map[ast.ASTHandle]map[string]uint64)}, nil
}

func (p *datalogParser) close() (err error) {
	for i := len(p.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, p.manager.DelRef(&p.pins[i]))
	}
	p.pins = nil
	return err
}

func (p *datalogParser) pin(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := p.manager.NewRef(h)
	if err == nil {
		p.pins = append(p.pins, r)
	}
	return h, err
}

func (p *datalogParser) advance() error {
	token, err := p.lexer.next()
	if err == nil {
		p.current = token
	}
	return err
}

func (p *datalogParser) parse(input []byte, path string) error {
	p.path = path
	p.lexer = newDatalogLexer(input)
	p.lexer.parsingDomains = true
	if err := p.advance(); err != nil {
		return err
	}
	if err := p.parseDomains(); err != nil {
		return err
	}
	p.lexer.parsingDomains = false
	return p.parseRules()
}

// ParseDatalogString parses the legacy Datalog syntax and adds declarations,
// facts and Horn rules to this context's fixedpoint engine.
func (c *Context) ParseDatalogString(input string) (resultErr error) {
	p, err := newDatalogParser(c)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.close()) }()
	if err = p.parse([]byte(input), ""); err != nil {
		return err
	}
	return c.rememberDatalogDisplay(p)
}

// ParseDatalogFile preserves source-relative .include resolution.
func (c *Context) ParseDatalogFile(filename string) (resultErr error) {
	input, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	p, err := newDatalogParser(c)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.close()) }()
	if err = p.parse(input, filepath.Dir(filename)); err != nil {
		return err
	}
	return c.rememberDatalogDisplay(p)
}
