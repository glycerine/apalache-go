package api

// Source: src/api/api_datalog.cpp:391-400.

import "strings"

func (f *APIFixedpoint) LoadSMT2String(input string) error {
	return f.LoadSMT2(strings.NewReader(input))
}
