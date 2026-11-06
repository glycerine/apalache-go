package sat

// Source: src/sat/dimacs.cpp:25-132.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func ReadDIMACSCNF(input io.Reader) (DIMACSCNF, error) {
	if input == nil {
		return DIMACSCNF{}, errors.Join(ErrDIMACSParse, io.ErrUnexpectedEOF)
	}
	reader := bufio.NewReader(input)
	var result DIMACSCNF
	var clause []DIMACSLiteral
	lineNumber := 0
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) != 0 {
			lineNumber++
			fields := strings.Fields(line)
			if len(fields) != 0 && (fields[0][0] == 'c' || fields[0][0] == 'p') {
				if len(clause) != 0 {
					return DIMACSCNF{}, fmt.Errorf("%w at line %d: directive inside clause", ErrDIMACSParse, lineNumber)
				}
			} else {
				for _, field := range fields {
					if len(clause) == 0 && (field[0] == 'c' || field[0] == 'p') {
						break
					}
					value, err := strconv.ParseInt(field, 10, 32)
					if err != nil {
						return DIMACSCNF{}, fmt.Errorf("%w at line %d: %q", ErrDIMACSParse, lineNumber, field)
					}
					if value == 0 {
						result.Clauses = append(result.Clauses, clause)
						clause = nil
						continue
					}
					magnitude := value
					if magnitude < 0 {
						magnitude = -magnitude
					}
					if uint32(magnitude) > result.VariableCount {
						result.VariableCount = uint32(magnitude)
					}
					clause = append(clause, DIMACSLiteral(value))
				}
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return DIMACSCNF{}, readErr
			}
			break
		}
	}
	if len(clause) != 0 {
		return DIMACSCNF{}, fmt.Errorf("%w: unterminated clause", ErrDIMACSParse)
	}
	return result, nil
}
