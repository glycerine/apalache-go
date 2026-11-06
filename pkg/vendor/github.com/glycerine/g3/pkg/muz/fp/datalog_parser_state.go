package fp

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type datalogParser struct {
	context      *base.DatalogContext
	manager      *ast.ASTManager
	lexer        *datalogLexer
	token        datalogToken
	path         string
	dir          string
	sorts        map[string]ast.ASTHandle
	vars         map[string]ast.ASTHandle
	argumentRefs []ast.ASTRef
	nextVar      uint32
}

func parseDatalog(context *base.DatalogContext, input io.Reader, path, dir string) error {
	if context == nil {
		return fmt.Errorf("muz/fp: nil Datalog context")
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	var lexer *datalogLexer
	if path != "" {
		lexer, err = newDatalogFileLexer(input)
	} else {
		lexer, err = newDatalogLexer(input)
	}
	if err != nil {
		return err
	}
	p := &datalogParser{context: context, manager: manager, lexer: lexer, path: path, dir: dir, sorts: make(map[string]ast.ASTHandle), vars: make(map[string]ast.ASTHandle)}
	p.advance()
	parseErr := p.parseProgram()
	cleanupErr := p.closeArguments()
	if len(p.argumentRefs) != 0 {
		return &datalogParserCleanupFailure{parser: p, cause: parseErr, cleanupErr: cleanupErr}
	}
	return joinErrors(parseErr, cleanupErr)
}

func (p *datalogParser) advance() { p.token = p.lexer.next() }

func (p *datalogParser) fail(message string) error {
	return &DatalogParseError{Path: p.path, Line: p.token.line, Column: p.token.column, Message: message}
}

func (p *datalogParser) expect(kind datalogTokenKind, message string) error {
	if p.token.kind != kind {
		return p.fail(message)
	}
	p.advance()
	return nil
}

func (p *datalogParser) symbol(text string) (usymbol.Symbol, error) {
	return p.manager.Symbols.Intern(text)
}

func (p *datalogParser) parseProgram() error {
	p.lexer.parsingDomains = true
	for p.token.kind == datalogIdentifier || p.token.kind == datalogInclude {
		if p.token.kind == datalogInclude {
			if err := p.parseInclude(true); err != nil {
				return err
			}
			continue
		}
		if err := p.parseDomain(); err != nil {
			return err
		}
	}
	if p.token.kind == datalogNewline {
		p.advance()
	}
	p.lexer.parsingDomains = false
	for p.token.kind != datalogEOF {
		if p.token.kind == datalogNewline {
			p.advance()
			continue
		}
		if p.token.kind == datalogInclude {
			if err := p.parseInclude(false); err != nil {
				return err
			}
			continue
		}
		if p.token.kind == datalogError {
			return p.fail(p.token.text)
		}
		if p.token.kind != datalogIdentifier {
			return p.fail("expected predicate declaration or rule")
		}
		if err := p.parseDeclarationOrRule(); err != nil {
			return err
		}
	}
	return nil
}

func (p *datalogParser) includePath(name string) string {
	if filepath.IsAbs(name) || p.dir == "" {
		return name
	}
	return filepath.Join(p.dir, name)
}

func domainBaseName(name string) (string, bool) {
	trimmed := strings.TrimRight(name, "0123456789")
	return trimmed, trimmed != ""
}
