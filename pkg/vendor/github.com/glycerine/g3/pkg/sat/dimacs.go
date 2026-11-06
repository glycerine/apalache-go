package sat

// Source: src/sat/dimacs.h; src/sat/dimacs.cpp.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

var ErrDIMACSUnexpectedChar = errors.New("sat: DIMACS unexpected character")

// dimacsStreamBuffer is the source one-character lookahead buffer. Any stream
// failure has the same observable effect as std::istream::get returning EOF.
type dimacsStreamBuffer struct {
	input *bufio.Reader
	value int
}

func newDIMACSStreamBuffer(input io.Reader) *dimacsStreamBuffer {
	b := &dimacsStreamBuffer{input: bufio.NewReader(input)}
	b.advance()
	return b
}

func (b *dimacsStreamBuffer) advance() {
	v, err := b.input.ReadByte()
	if err != nil {
		b.value = -1
		return
	}
	b.value = int(v)
}

func (b *dimacsStreamBuffer) skipWhitespace() {
	for (b.value >= 9 && b.value <= 13) || b.value == 32 {
		b.advance()
	}
}

func (b *dimacsStreamBuffer) skipLine() {
	for b.value != -1 {
		if b.value == '\n' {
			b.advance()
			return
		}
		b.advance()
	}
}

func (b *dimacsStreamBuffer) parseInt() (int, error) {
	b.skipWhitespace()
	negative := false
	switch b.value {
	case '-':
		negative = true
		b.advance()
	case '+':
		b.advance()
	}
	if b.value < '0' || b.value > '9' {
		return 0, fmt.Errorf("%w: %d", ErrDIMACSUnexpectedChar, b.value)
	}
	value := 0
	for b.value >= '0' && b.value <= '9' {
		value = value*10 + b.value - '0'
		b.advance()
	}
	if negative {
		return -value, nil
	}
	return value, nil
}

func (b *dimacsStreamBuffer) readClause(solver *Solver, literals []Literal) ([]Literal, error) {
	literals = literals[:0]
	for {
		parsed, err := b.parseInt()
		if err != nil {
			return literals, err
		}
		if parsed == 0 {
			return literals, nil
		}
		variable := parsed
		if variable < 0 {
			variable = -variable
		}
		// The source deliberately uses the absolute DIMACS number as its SAT
		// variable, leaving variable zero unused.
		n, err := solver.sourceNumVars()
		if err != nil {
			return literals, err
		}
		for uint64(variable) >= uint64(n) {
			if _, err = solver.MkVar(false, true); err != nil {
				return literals, err
			}
			n++
		}
		literals = append(literals, MakeLiteral(BoolVar(variable), parsed < 0))
	}
}

// ParseDIMACS appends the clauses read from input to solver. Header and comment
// lines are informational, as in the source parser. A parse or solver error
// leaves variables and clauses already published before the error intact.
//
// The C++ command-line parser prints malformed-input diagnostics and exits the
// process. The Go library reports the same offending character to its caller.
func ParseDIMACS(input io.Reader, solver *Solver) error {
	if input == nil || solver == nil {
		return ErrSATOwner
	}
	if err := solver.open(); err != nil {
		return err
	}
	buffer := newDIMACSStreamBuffer(input)
	literals := make([]Literal, 0)
	for {
		buffer.skipWhitespace()
		switch buffer.value {
		case -1:
			return nil
		case 'c', 'p':
			buffer.skipLine()
		default:
			var err error
			literals, err = buffer.readClause(solver, literals)
			if err != nil {
				return err
			}
			if err = solver.MkClause(literals); err != nil {
				return err
			}
		}
	}
}
