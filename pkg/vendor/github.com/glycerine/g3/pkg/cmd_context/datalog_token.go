package cmd_context

// Source: src/muz/fp/datalog_parser.cpp dtoken and reserved_symbols.

type datalogToken uint8

const (
	dlLeftParen datalogToken = iota
	dlRightParen
	dlString
	dlIdentifier
	dlNumber
	dlPeriod
	dlInclude
	dlComma
	dlColon
	dlWildcard
	dlLeftArrow
	dlEOF
	dlNewline
	dlNotEqual
	dlLess
	dlGreater
	dlEqual
	dlNegation
)

var datalogTokenNames = [...]string{"(", ")", "string", "identifier", "number", ".", ".include", ",", ":", "_", ":-", "end of input", "newline", "!=", "<", ">", "=", "!"}

type datalogLexeme struct {
	kind      datalogToken
	text      string
	line, col int
}
