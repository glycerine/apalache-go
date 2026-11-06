package fp

// Source: dparser::parse_include, parse_mapfile and line_reader.

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func joinErrors(left, right error) error { return errors.Join(left, right) }

func (p *datalogParser) parseInclude(domains bool) error {
	p.advance()
	if p.token.kind != datalogString {
		return p.fail("expected include file string")
	}
	path := p.includePath(p.token.text)
	p.advance()
	terminated := p.token.kind == datalogNewline
	if p.token.kind == datalogNewline {
		p.advance()
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	// Includes share the declaration and domain dictionaries but have their own
	// lexer, matching the source's scoped lexer replacement.
	lexer, err := newDatalogFileLexer(file)
	if err != nil {
		return err
	}
	child := *p
	// Includes run between declarations/rules. Keep any failed child cleanup
	// roots separate so the child can transfer only its own outstanding refs
	// back to the parent on every return path.
	child.argumentRefs = nil
	defer func() { p.argumentRefs = append(p.argumentRefs, child.argumentRefs...) }()
	child.lexer = lexer
	child.path = path
	child.dir = p.includePath(".")
	child.lexer.parsingDomains = domains
	child.advance()
	if domains {
		for child.token.kind == datalogIdentifier || child.token.kind == datalogInclude {
			if child.token.kind == datalogInclude {
				err = child.parseInclude(true)
			} else {
				err = child.parseDomain()
			}
			if err != nil {
				return err
			}
		}
	}
	child.lexer.parsingDomains = false
	for child.token.kind != datalogEOF {
		if child.token.kind == datalogNewline {
			child.advance()
			continue
		}
		if child.token.kind == datalogInclude {
			err = child.parseInclude(false)
		} else {
			err = child.parseDeclarationOrRule()
		}
		if err != nil {
			return err
		}
	}
	if domains && !terminated {
		return p.fail("newline expected after include statement")
	}
	return nil
}

func (p *datalogParser) loadMap(sort ast.ASTHandle, name string) (resultErr error) {
	file, err := os.Open(p.includePath(name))
	if err != nil {
		// Source parse_mapfile treats an unavailable optional map as a warning
		// and continues. The Go parser has no diagnostic stream, so preserve
		// the acceptance behavior without writing process-global stderr.
		return nil
	}
	// Source line_reader closes its FILE in the destructor and ignores fclose's
	// result. Do not turn a late close failure into a Datalog parse failure.
	defer file.Close()

	reader := bufio.NewReader(file)
	// Source line_reader returns the bytes up to '\n' verbatim, including a
	// final empty entry when the file ends immediately after a newline.
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		line = strings.TrimSuffix(line, "\n")
		symbol, err := p.symbol(line)
		if err != nil {
			return err
		}
		if _, err = p.context.SymbolNumber(sort, symbol); err != nil {
			return err
		}
		if readErr == io.EOF {
			return nil
		}
	}
}
