package cmd_context

// Source: src/api/api_datalog.cpp:347-358 and
// src/muz/base/dl_context.cpp display_smt2.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// WriteFixedpointSMT2 writes the source context dump, optionally with queries.
// Use WriteFixedpointReplaySMT2 to export all registered relations for replay.
func (c *Context) WriteFixedpointSMT2(out io.Writer, queries ...ast.ASTHandle) error {
	return c.WriteFixedpointSMT2WithQueries(out, queries)
}

// WriteFixedpointSMT2WithQueries writes the source context dump with queries
// in caller order and the effective engine/module print settings.
func (c *Context) WriteFixedpointSMT2WithQueries(out io.Writer, queries []ast.ASTHandle) error {
	return c.WriteFixedpointSMT2WithQueryParameters(out, queries, nil)
}

// WriteFixedpointSMT2WithQueryParameters applies borrowed fixedpoint-local
// print options without changing the live engine's query configuration.
func (c *Context) WriteFixedpointSMT2WithQueryParameters(out io.Writer, queries []ast.ASTHandle, parameters *uparams.ParamsRef) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.WriteContextSMT2(out, queries, parameters)
}
