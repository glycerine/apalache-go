package util

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

type Token uint8

const (
	LeftParen Token = iota + 1
	RightParen
	Colon
	Identifier
	String
	Comment
	Integer
	BitVector
	Float
	EndOfFile
	ScanError
)

type Parameter struct {
	Number *uint32
	Symbol string
}

type Scanner struct {
	reader         *bufio.Reader
	errors         io.Writer
	smt2, bvToken  bool
	line, position int
	id             string
	number         big.Rat
	bvSize         uint32
	params         []Parameter
	state          Token
}

func NewScanner(reader io.Reader, errors io.Writer, smt2, bitVectorToken bool) *Scanner {
	if errors == nil {
		errors = io.Discard
	}
	return &Scanner{reader: bufio.NewReader(reader), errors: errors, smt2: smt2, bvToken: bitVectorToken, line: 1, state: Identifier, bvSize: ^uint32(0)}
}

func (s *Scanner) Line() int               { return s.line }
func (s *Scanner) Position() int           { return s.position }
func (s *Scanner) ID() string              { return s.id }
func (s *Scanner) Number() *big.Rat        { return new(big.Rat).Set(&s.number) }
func (s *Scanner) BitVectorSize() uint32   { return s.bvSize }
func (s *Scanner) Parameters() []Parameter { return append([]Parameter(nil), s.params...) }
func (s *Scanner) healthy() bool           { return s.state != ScanError && s.state != EndOfFile }

func (s *Scanner) read() (rune, error) {
	r, _, err := s.reader.ReadRune()
	if err == nil {
		s.position++
	}
	return r, err
}
func (s *Scanner) unread() {
	if s.reader.UnreadRune() == nil {
		s.position--
	}
}

func (s *Scanner) report(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(s.errors, format, args...)
}

func (s *Scanner) readQuotedSymbol() Token {
	var value strings.Builder
	escaped := false
	for {
		r, err := s.read()
		if err == io.EOF {
			s.report("ERROR: unexpected end of file.\n")
			s.state = EndOfFile
			return s.state
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if r == '\n' {
			s.line++
			s.position = 0
		}
		if r == '|' && !escaped {
			s.id = value.String()
			return Identifier
		}
		value.WriteRune(r)
		escaped = r == '\\' && !escaped
		if r != '\\' {
			escaped = false
		}
	}
}

func smt2SymbolRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("~!@$%^&*_-+=<>.?/", r)
}
func legacySymbolRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_=<>+-*/%~&@#|\\.'!?", r)
}

func (s *Scanner) readIdentifier(first rune) Token {
	s.params = nil
	if first == '|' && s.smt2 {
		return s.readQuotedSymbol()
	}
	var value strings.Builder
	value.WriteRune(first)
	isArithmetic := !s.smt2 && strings.ContainsRune("=<>+-*/%~&@#|\\", first)
	isAlpha := unicode.IsLetter(first)
	for {
		r, err := s.read()
		if err == io.EOF {
			s.id = value.String()
			return Identifier
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if !s.smt2 && first == '-' && value.Len() == 1 && unicode.IsDigit(r) {
			return s.readNumber(r, false)
		}
		allowed := smt2SymbolRune(r)
		if !s.smt2 {
			allowed = legacySymbolRune(r)
		}
		if r == '[' {
			s.id = value.String()
			if s.readParameters() {
				return Identifier
			}
			return s.state
		}
		if isArithmetic && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ':' || r == '.') {
			allowed = false
		}
		if !s.smt2 && !isArithmetic && strings.ContainsRune("=<>+*/%~&@#|\\", r) {
			allowed = false
		}
		if !s.smt2 && !isArithmetic && r == '-' && !isAlpha {
			allowed = false
		}
		if !allowed {
			s.unread()
			s.id = value.String()
			return Identifier
		}
		value.WriteRune(r)
	}
}

func (s *Scanner) readParameters() bool {
	var value strings.Builder
	flush := func() bool {
		text := value.String()
		value.Reset()
		if n, err := strconv.ParseUint(text, 10, 32); err == nil && text != "" {
			v := uint32(n)
			s.params = append(s.params, Parameter{Number: &v})
		} else {
			s.params = append(s.params, Parameter{Symbol: text})
		}
		return true
	}
	for {
		r, err := s.read()
		if err != nil {
			s.report("ERROR: unexpected end of file.\n")
			s.state = ScanError
			return false
		}
		switch r {
		case ':':
			flush()
		case ']':
			flush()
			return true
		default:
			value.WriteRune(r)
		}
	}
}

func (s *Scanner) readNumber(first rune, positive bool) Token {
	var value strings.Builder
	value.WriteRune(first)
	fraction := false
	for {
		r, err := s.read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if unicode.IsDigit(r) {
			value.WriteRune(r)
			continue
		}
		if r == '.' {
			fraction = true
			value.WriteRune(r)
			continue
		}
		s.unread()
		break
	}
	text := value.String()
	if strings.HasSuffix(text, ".") {
		text += "0"
	}
	if !positive {
		text = "-" + text
	}
	if _, ok := s.number.SetString(text); !ok {
		s.state = ScanError
		return s.state
	}
	if fraction {
		return Float
	}
	return Integer
}

func (s *Scanner) readDelimited(delimiter rune, token Token) Token {
	var value strings.Builder
	for {
		r, err := s.read()
		if err == io.EOF {
			s.id = value.String()
			return token
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if r == '\n' {
			s.line++
			s.position = 0
		}
		if r == delimiter {
			s.id = value.String()
			return token
		}
		if r == '\\' {
			value.WriteRune(r)
			next, readErr := s.read()
			if readErr != nil {
				s.id = value.String()
				return token
			}
			value.WriteRune(next)
			continue
		}
		value.WriteRune(r)
	}
}

func (s *Scanner) readBitVector() Token {
	kind, err := s.read()
	if err != nil {
		s.state = ScanError
		return s.state
	}
	if !s.bvToken {
		prefix, valid := "bvbin", kind == 'b'
		if kind == 'x' {
			prefix, valid = "bvhex", true
		}
		if !valid {
			s.report("ERROR: unexpected character after '#': '%c'.\n", kind)
			s.state = ScanError
			return s.state
		}
		var value strings.Builder
		value.WriteString(prefix)
		for {
			r, readErr := s.read()
			if readErr != nil {
				break
			}
			validDigit := r == '0' || r == '1' || kind == 'x' && (unicode.IsDigit(r) || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F')
			if !validDigit {
				s.unread()
				break
			}
			value.WriteRune(r)
		}
		s.id = value.String()
		return Identifier
	}
	base, width := 0, uint32(0)
	if kind == 'x' {
		base = 16
	} else if kind == 'b' {
		base = 2
	} else {
		s.state = ScanError
		return s.state
	}
	value := new(big.Int)
	for {
		r, readErr := s.read()
		if readErr != nil {
			break
		}
		digit := -1
		switch {
		case r >= '0' && r <= '9':
			digit = int(r - '0')
		case r >= 'a' && r <= 'f':
			digit = int(r-'a') + 10
		case r >= 'A' && r <= 'F':
			digit = int(r-'A') + 10
		}
		if digit < 0 || digit >= base {
			s.unread()
			break
		}
		value.Mul(value, big.NewInt(int64(base)))
		value.Add(value, big.NewInt(int64(digit)))
		if base == 16 {
			width += 4
		} else {
			width++
		}
	}
	if width == 0 {
		s.state = ScanError
		return s.state
	}
	s.number.SetInt(value)
	s.bvSize = width
	return BitVector
}

func (s *Scanner) Scan() Token {
	if !s.healthy() {
		return s.state
	}
	for {
		r, err := s.read()
		if err == io.EOF {
			s.state = EndOfFile
			return s.state
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		switch r {
		case ' ', '\t', '\r':
			continue
		case '\n':
			s.line++
			s.position = 0
			continue
		case ';':
			for {
				c, e := s.read()
				if e != nil {
					s.state = EndOfFile
					return s.state
				}
				if c == '\n' {
					s.line++
					s.position = 0
					break
				}
			}
		case ':':
			return Colon
		case '(':
			return LeftParen
		case ')':
			return RightParen
		case '{':
			return s.readDelimited('}', Comment)
		case '"':
			return s.readDelimited('"', String)
		case '#':
			if s.smt2 {
				return s.readBitVector()
			}
			return s.readIdentifier(r)
		default:
			if unicode.IsDigit(r) {
				return s.readNumber(r, true)
			}
			if unicode.IsLetter(r) || r == '?' || r == '$' || r == '.' || r == '|' || strings.ContainsRune("~!@$%^&*_-+=<>?/\\'#", r) {
				return s.readIdentifier(r)
			}
			s.report("ERROR: unexpected character: '%c'.\n", r)
			s.state = ScanError
			return s.state
		}
	}
}
