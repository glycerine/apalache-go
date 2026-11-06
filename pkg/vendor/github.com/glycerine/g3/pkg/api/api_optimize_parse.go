package api

// Source: src/api/api_opt.cpp:320-389.

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func (o *APIOptimize) Parse(input io.Reader) error {
	if err := o.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	return smt2.ParseAssertions(o.command, input)
}

func (o *APIOptimize) ParseString(input string) error {
	return o.Parse(strings.NewReader(input))
}

func (o *APIOptimize) ParseFile(path string) (resultErr error) {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	return o.Parse(input)
}
