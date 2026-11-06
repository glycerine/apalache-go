package cmd_context

// Source: src/muz/fp/datalog_parser.cpp dlexer. Input is retained as bytes;
// Datalog identifiers are byte strings in the source lexer too.

import "fmt"

type datalogLexer struct {
	input          []byte
	index          int
	line, column   int
	parsingDomains bool
}

func newDatalogLexer(input []byte) *datalogLexer {
	return &datalogLexer{input: input, line: 1, column: 1}
}

func (l *datalogLexer) atEnd() bool { return l.index >= len(l.input) }
func (l *datalogLexer) peek() byte {
	if l.atEnd() {
		return 0
	}
	return l.input[l.index]
}
func (l *datalogLexer) take() byte {
	c := l.peek()
	if l.atEnd() {
		return 0
	}
	l.index++
	if c == '\n' {
		l.line, l.column = l.line+1, 1
	} else {
		l.column++
	}
	return c
}

func dlSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' }
func dlDigit(c byte) bool { return c >= '0' && c <= '9' }

func (l *datalogLexer) next() (datalogLexeme, error) {
	for {
		if l.atEnd() {
			return datalogLexeme{kind: dlEOF, line: l.line, col: l.column}, nil
		}
		if dlSpace(l.peek()) {
			l.take()
			continue
		}
		if l.peek() == '#' {
			lineComment := l.column == 1
			for !l.atEnd() && l.peek() != '\n' {
				l.take()
			}
			// Source read_comment consumes the terminator only for a comment
			// beginning in the first physical column. In particular, such a
			// comment does not terminate the domain-declaration section.
			if lineComment && l.peek() == '\n' {
				l.take()
			}
			continue
		}
		if l.peek() == '\\' {
			line, col := l.line, l.column
			l.take()
			for l.peek() == ' ' {
				l.take()
			}
			if l.peek() == '\n' {
				l.take()
				continue
			}
			return l.readIdentifier("\\", line, col), nil
		}
		break
	}
	line, col := l.line, l.column
	switch c := l.take(); c {
	case '\n':
		return datalogLexeme{kind: dlNewline, line: line, col: col}, nil
	case '(':
		return datalogLexeme{kind: dlLeftParen, text: "(", line: line, col: col}, nil
	case ')':
		return datalogLexeme{kind: dlRightParen, text: ")", line: line, col: col}, nil
	case ',':
		return datalogLexeme{kind: dlComma, text: ",", line: line, col: col}, nil
	case '=':
		return datalogLexeme{kind: dlEqual, text: "=", line: line, col: col}, nil
	case '<':
		return datalogLexeme{kind: dlLess, text: "<", line: line, col: col}, nil
	case '>':
		return datalogLexeme{kind: dlGreater, text: ">", line: line, col: col}, nil
	case '!':
		if l.peek() == '=' {
			l.take()
			return datalogLexeme{kind: dlNotEqual, text: "!=", line: line, col: col}, nil
		}
		return datalogLexeme{kind: dlNegation, text: "!", line: line, col: col}, nil
	case ':':
		if l.peek() == '-' {
			l.take()
			return datalogLexeme{kind: dlLeftArrow, text: ":-", line: line, col: col}, nil
		}
		return datalogLexeme{kind: dlColon, text: ":", line: line, col: col}, nil
	case '"':
		start := l.index
		for !l.atEnd() && l.peek() != '"' && l.peek() != '\n' {
			l.take()
		}
		if l.atEnd() || l.peek() != '"' {
			return datalogLexeme{}, fmt.Errorf("datalog:%d:%d: unterminated string", line, col)
		}
		text := string(l.input[start:l.index])
		l.take()
		return datalogLexeme{kind: dlString, text: text, line: line, col: col}, nil
	case '|':
		start := l.index
		for !l.atEnd() && l.peek() != '|' {
			l.take()
		}
		if l.atEnd() {
			return datalogLexeme{}, fmt.Errorf("datalog:%d:%d: unterminated quoted identifier", line, col)
		}
		text := string(l.input[start:l.index])
		l.take()
		return datalogLexeme{kind: dlIdentifier, text: text, line: line, col: col}, nil
	default:
		prefix := string([]byte{c})
		if dlDigit(c) || (c == '-' && dlDigit(l.peek())) {
			return l.readNumber(prefix, line, col), nil
		}
		return l.readIdentifier(prefix, line, col), nil
	}
}

func (l *datalogLexer) readNumber(prefix string, line, col int) datalogLexeme {
	start := l.index
	for dlDigit(l.peek()) {
		l.take()
	}
	return datalogLexeme{kind: dlNumber, text: prefix + string(l.input[start:l.index]), line: line, col: col}
}

func (l *datalogLexer) readIdentifier(prefix string, line, col int) datalogLexeme {
	start := l.index
	for !l.atEnd() {
		c := l.peek()
		if c == '(' || c == ')' || c == '#' || c == ',' || c == ':' || c == '=' || c == '!' || c == '<' || c == '>' || c == '\n' || dlSpace(c) || (!l.parsingDomains && c == '.') {
			break
		}
		l.take()
	}
	text := prefix + string(l.input[start:l.index])
	kind := dlIdentifier
	switch text {
	case ".":
		kind = dlPeriod
	case ".include":
		kind = dlInclude
	case "_":
		kind = dlWildcard
	}
	return datalogLexeme{kind: kind, text: text, line: line, col: col}
}
