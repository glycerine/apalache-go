package smt2

// Source: src/parsers/smt2/smt2scanner.cpp and smt2scanner.h.

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type Token uint8

const (
	NullToken Token = iota
	LeftParenToken
	RightParenToken
	KeywordToken
	SymbolToken
	StringToken
	IntegerToken
	BitVectorToken
	DecimalToken
	EOFToken
)

// ScannerError records the source scanner exception's message and location.
type ScannerError struct {
	Message      string
	Line, Column int
}

func (e *ScannerError) Error() string {
	return fmt.Sprintf("line %d column %d: %s", e.Line, e.Column, e.Message)
}

// Scanner implements the SMT-LIB command parser's byte-level lexer. Go buffered
// input replaces the source stream buffer; token spelling and arithmetic stay
// independent of the AST manager until the parser consumes them.
type Scanner struct {
	reader                 *bufio.Reader
	compliant              bool
	curr                   byte
	eof                    bool
	line, column, position int
	id, text               string
	number                 big.Rat
	bvSize                 uint32
	caching                bool
	cache                  []byte
	traceTags              *diagnostic.TraceTags
}

func NewScanner(reader io.Reader, compliant bool) (*Scanner, error) {
	if reader == nil {
		return nil, fmt.Errorf("nil SMT-LIB input")
	}
	s := &Scanner{reader: bufio.NewReader(reader), compliant: compliant, line: 1, bvSize: ^uint32(0)}
	return s, s.next()
}

func (s *Scanner) Line() int             { return s.line }
func (s *Scanner) Position() int         { return s.position }
func (s *Scanner) ID() string            { return s.id }
func (s *Scanner) String() string        { return s.text }
func (s *Scanner) Number() *big.Rat      { return new(big.Rat).Set(&s.number) }
func (s *Scanner) BitVectorSize() uint32 { return s.bvSize }
func (s *Scanner) StartCaching()         { s.caching = true; s.cache = nil }
func (s *Scanner) StopCaching()          { s.caching = false }
func (s *Scanner) ResetCache()           { s.cache = nil }
func (s *Scanner) CacheSize() int        { return len(s.cache) }

func scannerSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\v' || c == '\f'
}

func (s *Scanner) CachedString(begin, end int) (string, error) {
	if begin < 0 || end < begin || end > len(s.cache) {
		return "", fmt.Errorf("invalid SMT-LIB cache range")
	}
	for begin < end && scannerSpace(s.cache[begin]) {
		begin++
	}
	for begin < end && scannerSpace(s.cache[end-1]) {
		end--
	}
	return string(s.cache[begin:end]), nil
}

func (s *Scanner) next() error {
	if s.eof {
		return &ScannerError{"unexpected end of input", s.line, s.column}
	}
	if s.caching {
		s.cache = append(s.cache, s.curr)
	}
	c, err := s.reader.ReadByte()
	s.column++
	if err == io.EOF {
		s.eof = true
		return nil
	}
	if err != nil {
		return err
	}
	s.curr = c
	return nil
}

func scannerSymbol(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("~!@$%^&*_-+=<>.?/", c) >= 0
}

func (s *Scanner) readSymbol(prefix string) (Token, error) {
	var text strings.Builder
	text.WriteString(prefix)
	for !s.eof && scannerSymbol(s.curr) {
		text.WriteByte(s.curr)
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	// The source returns EOF rather than publishing an unterminated final
	// symbol. Complete SMT-LIB commands supply the closing delimiter.
	if s.eof {
		return EOFToken, nil
	}
	s.id = text.String()
	return SymbolToken, s.trace("read_symbol_core", 105, "new symbol: "+s.id+"\n")
}

func (s *Scanner) readQuotedSymbol() (Token, error) {
	if err := s.next(); err != nil {
		return NullToken, err
	}
	var text strings.Builder
	escape := false
	for !s.eof {
		c := s.curr
		if c == '\n' {
			s.line++
			s.column = 0
		}
		if c == '|' && !escape {
			if err := s.next(); err != nil {
				return NullToken, err
			}
			s.id = text.String()
			return SymbolToken, s.trace("read_quoted_symbol", 85, "new quoted symbol: "+s.id+"\n")
		}
		escape = c == '\\'
		text.WriteByte(c)
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	return NullToken, &ScannerError{"unexpected end of quoted symbol", s.line, s.column}
}

func (s *Scanner) readNumber(negative bool) (Token, error) {
	var numerator big.Int
	denominator := big.NewInt(1)
	ten := big.NewInt(10)
	decimal := false
	for !s.eof {
		c := s.curr
		if c >= '0' && c <= '9' {
			numerator.Mul(&numerator, ten)
			numerator.Add(&numerator, big.NewInt(int64(c-'0')))
			if decimal {
				denominator.Mul(denominator, ten)
			}
		} else if c == '.' && !decimal {
			decimal = true
		} else {
			break
		}
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	s.number.SetFrac(&numerator, denominator)
	if err := s.trace("read_number", 147, "new number: "+s.number.RatString()+"\n"); err != nil {
		return NullToken, err
	}
	if negative {
		s.number.Neg(&s.number)
	}
	if decimal {
		return DecimalToken, nil
	}
	return IntegerToken, nil
}

func (s *Scanner) readString() (Token, error) {
	if err := s.next(); err != nil {
		return NullToken, err
	}
	var text strings.Builder
	for !s.eof {
		c := s.curr
		if c == '\n' {
			s.line++
			s.column = 0
		}
		if c == '"' {
			if err := s.next(); err != nil {
				return NullToken, err
			}
			if s.eof || s.curr != '"' {
				s.text = text.String()
				return StringToken, nil
			}
		}
		text.WriteByte(c)
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	return NullToken, &ScannerError{"unexpected end of string", s.line, s.column}
}

func scannerDigit(c byte, base int64) int64 {
	v := int64(-1)
	if c >= '0' && c <= '9' {
		v = int64(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		v = int64(c-'a') + 10
	}
	if c >= 'A' && c <= 'F' {
		v = int64(c-'A') + 10
	}
	if v >= base {
		return -1
	}
	return v
}

func (s *Scanner) readBitVector() (Token, error) {
	if err := s.next(); err != nil {
		return NullToken, err
	}
	base, width := int64(0), uint32(0)
	if !s.eof && s.curr == 'x' {
		base, width = 16, 4
	}
	if !s.eof && s.curr == 'b' {
		base, width = 2, 1
	}
	if base == 0 {
		return NullToken, &ScannerError{"invalid bit-vector literal, expecting 'x' or 'b'", s.line, s.column}
	}
	if err := s.next(); err != nil {
		return NullToken, err
	}
	var value big.Int
	s.number.SetInt64(0)
	s.bvSize = 0
	for !s.eof {
		digit := scannerDigit(s.curr, base)
		if digit < 0 {
			break
		}
		value.Mul(&value, big.NewInt(base))
		value.Add(&value, big.NewInt(digit))
		s.bvSize += width
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	if s.bvSize == 0 {
		return NullToken, &ScannerError{"invalid empty bit-vector literal", s.line, s.column}
	}
	s.number.SetInt(&value)
	return BitVectorToken, nil
}

func (s *Scanner) trace(function string, line int, text string) error {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	return tags.Emit("scanner", function, "src/parsers/smt2/smt2scanner.cpp", line, text)
}

func (s *Scanner) Scan() (Token, error) {
	for !s.eof {
		s.position = s.column
		c := s.curr
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			if err := s.next(); err != nil {
				return NullToken, err
			}
		case c == '\n':
			if err := s.next(); err != nil {
				return NullToken, err
			}
			s.line++
			s.column = 0
		case c == ';':
			for !s.eof && s.curr != '\n' {
				if err := s.next(); err != nil {
					return NullToken, err
				}
			}
			if !s.eof {
				s.line++
				s.column = 0
				if err := s.next(); err != nil {
					return NullToken, err
				}
			}
		case c == '(' || c == ')':
			if err := s.next(); err != nil {
				return NullToken, err
			}
			if c == '(' {
				return LeftParenToken, nil
			}
			return RightParenToken, nil
		case c == ':':
			if err := s.next(); err != nil {
				return NullToken, err
			}
			_, err := s.readSymbol(":")
			return KeywordToken, err
		case c == '|':
			return s.readQuotedSymbol()
		case c == '"':
			return s.readString()
		case c == '#':
			return s.readBitVector()
		case c >= '0' && c <= '9':
			return s.readNumber(false)
		case c == '-' && !s.compliant:
			if err := s.next(); err != nil {
				return NullToken, err
			}
			if !s.eof && s.curr >= '0' && s.curr <= '9' {
				return s.readNumber(true)
			}
			return s.readSymbol("-")
		case scannerSymbol(c):
			return s.readSymbol("")
		default:
			err := &ScannerError{"unexpected character", s.line, s.column}
			if next := s.next(); next != nil {
				return NullToken, next
			}
			return NullToken, err
		}
	}
	s.position = s.column
	return EOFToken, nil
}
