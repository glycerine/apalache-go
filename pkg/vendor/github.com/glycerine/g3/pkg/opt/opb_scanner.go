package opt

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
)

type opbScanner struct {
	reader *bufio.Reader
	line   int
}

func newOPBScanner(input io.Reader) *opbScanner {
	return &opbScanner{reader: bufio.NewReader(input), line: 1}
}

func (s *opbScanner) read() (byte, error) {
	b, err := s.reader.ReadByte()
	if err == nil && b == '\n' {
		s.line++
	}
	return b, err
}

func (s *opbScanner) peek() (byte, error) {
	b, err := s.reader.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (s *opbScanner) skipWhitespace() error {
	for {
		b, err := s.peek()
		if err != nil {
			return err
		}
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			return nil
		}
		_, _ = s.read()
	}
}

func (s *opbScanner) consume(text string) (bool, error) {
	if err := s.skipWhitespace(); err != nil {
		return false, err
	}
	bytes, err := s.reader.Peek(len(text))
	if err != nil {
		return false, err
	}
	if string(bytes) != text {
		return false, nil
	}
	for range text {
		_, _ = s.read()
	}
	return true, nil
}

func (s *opbScanner) integer() (big.Int, error) {
	var result big.Int
	if err := s.skipWhitespace(); err != nil {
		return result, fmt.Errorf("opb:%d: expected integer: %w", s.line, err)
	}
	sign := byte(0)
	if b, _ := s.peek(); b == '+' || b == '-' {
		sign, _ = s.read()
		if err := s.skipWhitespace(); err != nil {
			return result, fmt.Errorf("opb:%d: expected digits: %w", s.line, err)
		}
	}
	digits := make([]byte, 0, 16)
	for {
		b, err := s.peek()
		if err != nil || b < '0' || b > '9' {
			break
		}
		b, _ = s.read()
		digits = append(digits, b)
	}
	if len(digits) == 0 {
		return result, fmt.Errorf("opb:%d: expected integer", s.line)
	}
	if sign == '-' {
		digits = append([]byte{'-'}, digits...)
	}
	if _, ok := result.SetString(string(digits), 10); !ok {
		return result, fmt.Errorf("opb:%d: invalid integer", s.line)
	}
	return result, nil
}

func (s *opbScanner) skipComment() error {
	for {
		b, err := s.read()
		if err != nil || b == '\n' {
			return err
		}
	}
}
