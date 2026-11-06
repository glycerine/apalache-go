package api

// Source: src/api/api_datalog.cpp:403-415.

import (
	"errors"
	"os"
)

func (f *APIFixedpoint) LoadSMT2File(path string) (resultErr error) {
	if err := f.open(); err != nil {
		return err
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	return f.LoadSMT2(input)
}
