package api

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type z3ReplayScanner struct {
	reader  *bufio.Reader
	current byte
	line    uint32
	eof     bool
}

func newZ3ReplayScanner(r *bufio.Reader) *z3ReplayScanner {
	s := &z3ReplayScanner{reader: r, line: 1}
	s.next()
	return s
}

func (s *z3ReplayScanner) next() {
	b, err := s.reader.ReadByte()
	if err != nil {
		s.eof = true
		return
	}
	s.current = b
}

func (s *z3ReplayScanner) blanks() {
	for !s.eof {
		if s.current == '\n' {
			s.line++
			s.next()
		} else if s.current == ' ' || s.current == '\t' || s.current == '\r' {
			s.next()
		} else {
			return
		}
	}
}

func (s *z3ReplayScanner) quoted(delimiter byte) (string, error) {
	if s.eof || s.current != delimiter {
		return "", fmt.Errorf("invalid string/symbol")
	}
	s.next()
	value := make([]byte, 0, 32)
	for !s.eof {
		c := s.current
		if c == '\n' {
			return "", fmt.Errorf("unexpected end of line")
		}
		if c == delimiter {
			s.next()
			return string(value), nil
		}
		if c == '\\' {
			s.next()
			v := 0
			for i := 0; i < 3; i++ {
				if s.eof || s.current < '0' || s.current > '9' {
					return "", fmt.Errorf("invalid escaped character")
				}
				v = 10*v + int(s.current-'0')
				if v > 255 {
					return "", fmt.Errorf("invalid escaped character")
				}
				s.next()
			}
			value = append(value, byte(v))
			continue
		}
		value = append(value, c)
		s.next()
	}
	return "", io.ErrUnexpectedEOF
}

func (s *z3ReplayScanner) digits(signed bool) (string, error) {
	result := make([]byte, 0, 20)
	if signed && !s.eof && s.current == '-' {
		result = append(result, '-')
		s.next()
	}
	start := len(result)
	for !s.eof && s.current >= '0' && s.current <= '9' {
		result = append(result, s.current)
		s.next()
	}
	if len(result) == start {
		return "", fmt.Errorf("invalid integer")
	}
	return string(result), nil
}

func (s *z3ReplayScanner) uint64() (uint64, error) {
	text, err := s.digits(false)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(text, 10, 64)
}

func (s *z3ReplayScanner) int64() (int64, error) {
	text, err := s.digits(true)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(text, 10, 64)
}

func (s *z3ReplayScanner) hexadecimal() (uint64, error) {
	text := make([]byte, 0, 16)
	for !s.eof {
		c := s.current
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || (len(text) == 1 && (c == 'x' || c == 'X')) {
			text = append(text, c)
			s.next()
			continue
		}
		break
	}
	if len(text) == 0 {
		return 0, fmt.Errorf("invalid pointer")
	}
	value := string(text)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	if value == "" {
		return 0, fmt.Errorf("invalid pointer")
	}
	return strconv.ParseUint(value, 16, 64)
}

func (s *z3ReplayScanner) floating(bits int) (float64, error) {
	text := make([]byte, 0, 24)
	for !s.eof {
		c := s.current
		if c == '-' || c == '.' || c == 'e' || c == 'E' || (c >= '0' && c <= '9') {
			text = append(text, c)
			s.next()
		} else {
			break
		}
	}
	if len(text) == 0 {
		return 0, fmt.Errorf("invalid floating point value")
	}
	return strconv.ParseFloat(string(text), bits)
}
