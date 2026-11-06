package fp

// Source: src/muz/fp/datalog_parser.cpp dtoken, reserved_symbols and dlexer.

import (
	"io"
	"strings"
	"unicode"
)

type datalogTokenKind uint8

const (
	datalogError datalogTokenKind = iota
	datalogEOF
	datalogNewline
	datalogLeftParen
	datalogRightParen
	datalogString
	datalogIdentifier
	datalogNumber
	datalogPeriod
	datalogInclude
	datalogComma
	datalogColon
	datalogWildcard
	datalogLeftArrow
	datalogNotEqual
	datalogLess
	datalogGreater
	datalogEqual
	datalogNegation
)

type datalogToken struct {
	kind         datalogTokenKind
	text         string
	line, column int
}

// Source reserved_symbols is also applied to decoded bar-quoted tokens.
func datalogReservedToken(text string) datalogTokenKind {
	switch text {
	case ":-":
		return datalogLeftArrow
	case "_":
		return datalogWildcard
	case ".":
		return datalogPeriod
	case "!=":
		return datalogNotEqual
	case "=":
		return datalogEqual
	case "<":
		return datalogLess
	case ">":
		return datalogGreater
	case ":":
		return datalogColon
	case ".include":
		return datalogInclude
	case "!":
		return datalogNegation
	default:
		return datalogIdentifier
	}
}

func datalogTokenText(text string) string {
	if index := strings.IndexByte(text, 0); index >= 0 {
		return text[:index]
	}
	return text
}

type datalogLexer struct {
	input []rune
	// Source char_reader marks EOF as soon as it loads the final physical
	// line; read_string observes that state before consuming each character.
	fileEOFAt      []bool
	position       int
	line, column   int
	parsingDomains bool
}

func newDatalogLexer(input io.Reader) (*datalogLexer, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	return &datalogLexer{input: []rune(string(data)), line: 1, column: 1}, nil
}

// The source file parser uses char_reader over line_reader: every returned
// C-string line is followed by a synthetic newline, including its final
// empty line. The stream parser instead reads the input directly.
func newDatalogFileLexer(input io.Reader) (*datalogLexer, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	var text strings.Builder
	var eofAt []bool
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		line = datalogTokenText(line)
		text.WriteString(line)
		for range line {
			eofAt = append(eofAt, index == len(lines)-1)
		}
		text.WriteByte('\n')
		eofAt = append(eofAt, index == len(lines)-1)
	}
	lexer, err := newDatalogLexer(strings.NewReader(text.String()))
	if err != nil {
		return nil, err
	}
	lexer.fileEOFAt = eofAt
	return lexer, nil
}

func newStringReader(input string) io.Reader { return strings.NewReader(input) }

func (l *datalogLexer) peek() rune {
	if l.position == len(l.input) {
		return -1
	}
	return l.input[l.position]
}

func (l *datalogLexer) take() rune {
	r := l.peek()
	if r < 0 {
		return r
	}
	l.position++
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return r
}

func (l *datalogLexer) next() datalogToken {
	for l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\r' {
		l.take()
	}
	line, column := l.line, l.column
	r := l.take()
	identifierPrefix := string(r)
	switch r {
	case -1:
		return datalogToken{kind: datalogEOF, line: line, column: column}
	case '\n':
		return datalogToken{kind: datalogNewline, text: "\\n", line: line, column: column}
	case '\\':
		// Source lookahead_newline ignores spaces after a continuation
		// backslash. A following column-reset comment is part of the same
		// continuation; otherwise the backslash begins an ordinary identifier.
		var prefix strings.Builder
		prefix.WriteRune(r)
		for l.peek() == ' ' {
			prefix.WriteRune(l.take())
		}
		if l.peek() == '\n' {
			l.take()
			return l.next()
		}
		if l.peek() == '#' {
			for l.peek() >= 0 && l.peek() != '\n' {
				l.take()
			}
			if l.peek() == '\n' {
				l.take()
			}
			return l.next()
		}
		identifierPrefix = prefix.String()
	case '#':
		lineComment := column == 1
		for l.peek() >= 0 && l.peek() != '\n' {
			l.take()
		}
		// Source read_comment consumes the newline only when '#' begins the
		// physical line. This lets comment-only lines occur among domains,
		// while a trailing comment still exposes the line terminator.
		if lineComment && l.peek() == '\n' {
			l.take()
		}
		return l.next()
	case '(':
		return datalogToken{kind: datalogLeftParen, text: "(", line: line, column: column}
	case ')':
		return datalogToken{kind: datalogRightParen, text: ")", line: line, column: column}
	case ',':
		return datalogToken{kind: datalogComma, text: ",", line: line, column: column}
	case ':':
		if l.peek() == '-' {
			l.take()
			return datalogToken{kind: datalogLeftArrow, text: ":-", line: line, column: column}
		}
		return datalogToken{kind: datalogColon, text: ":", line: line, column: column}
	case '!':
		if l.peek() == '=' {
			l.take()
			return datalogToken{kind: datalogNotEqual, text: "!=", line: line, column: column}
		}
		return datalogToken{kind: datalogNegation, text: "!", line: line, column: column}
	case '=':
		return datalogToken{kind: datalogEqual, text: "=", line: line, column: column}
	case '"':
		var value strings.Builder
		for l.peek() >= 0 && l.peek() != '"' && l.peek() != '\n' {
			if len(l.fileEOFAt) != 0 && l.fileEOFAt[l.position] {
				return datalogToken{kind: datalogError, text: "unterminated string", line: line, column: column}
			}
			value.WriteRune(l.take())
		}
		if l.peek() != '"' {
			return datalogToken{kind: datalogError, text: "unterminated string", line: line, column: column}
		}
		l.take()
		return datalogToken{kind: datalogString, text: datalogTokenText(value.String()), line: line, column: column}
	case '|':
		var value strings.Builder
		for l.peek() >= 0 && l.peek() != '|' {
			value.WriteRune(l.take())
		}
		if l.peek() == '|' {
			l.take()
		}
		text := datalogTokenText(value.String())
		return datalogToken{kind: datalogReservedToken(text), text: text, line: line, column: column}
	}
	if unicode.IsDigit(r) || r == '-' && unicode.IsDigit(l.peek()) {
		var value strings.Builder
		value.WriteRune(r)
		for unicode.IsDigit(l.peek()) {
			value.WriteRune(l.take())
		}
		return datalogToken{kind: datalogNumber, text: value.String(), line: line, column: column}
	}
	var value strings.Builder
	value.WriteString(identifierPrefix)
	for {
		p := l.peek()
		if p < 0 || unicode.IsSpace(p) || strings.ContainsRune("(),:#=", p) || (!l.parsingDomains && p == '.') {
			break
		}
		value.WriteRune(l.take())
	}
	text := datalogTokenText(value.String())
	return datalogToken{kind: datalogReservedToken(text), text: text, line: line, column: column}
}
