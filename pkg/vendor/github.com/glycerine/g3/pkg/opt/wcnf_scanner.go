package opt

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

type wcnfScanner struct {
	input *bufio.Scanner
	line  int
}

func newWCNFScanner(input io.Reader) *wcnfScanner {
	s := bufio.NewScanner(input)
	s.Buffer(make([]byte, 1024), 1024*1024)
	return &wcnfScanner{input: s}
}

func (s *wcnfScanner) scan() bool { s.line++; return s.input.Scan() }

func (s *wcnfScanner) errorf(format string, args ...interface{}) error {
	return fmt.Errorf("wcnf:%d: %s", s.line, fmt.Sprintf(format, args...))
}

func (s *wcnfScanner) uint(text string, what string) (uint64, error) {
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, s.errorf("invalid %s %q", what, text)
	}
	return v, nil
}
