package api

// Source: src/api/api_parsers.cpp:110-148.

import (
	"bytes"
	"strings"
)

// EvaluateSMT2 executes commands in the owning persistent command session and
// returns exactly the regular and diagnostic output produced by that input.
func (c *SMTContext) EvaluateSMT2(input string) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	var output bytes.Buffer
	err := c.ExecuteSMT2(strings.NewReader(input), &output)
	return output.String(), err
}
