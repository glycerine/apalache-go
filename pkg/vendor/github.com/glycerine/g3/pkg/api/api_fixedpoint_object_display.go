package api

// Source: src/api/api_datalog.cpp:347-358.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (f *APIFixedpoint) WriteSMT2(output io.Writer, queries ...ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	if output == nil {
		return io.ErrClosedPipe
	}
	var local *uparams.ParamsRef
	if f.engineParams != nil {
		local = &f.engineParams.value
	}
	return f.command.WriteFixedpointSMT2WithQueryParameters(output, queries, local)
}

func (f *APIFixedpoint) SMT2(queries ...ast.ASTHandle) (string, error) {
	if err := f.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	var local *uparams.ParamsRef
	if f.engineParams != nil {
		local = &f.engineParams.value
	}
	if err := f.command.WriteFixedpointSMT2WithQueryParameters(&output, queries, local); err != nil {
		return "", err
	}
	return output.String(), nil
}
