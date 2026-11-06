package smt2

// Source: src/parsers/smt2/marshal.h; src/parsers/smt2/marshal.cpp.

import "errors"

var ErrMarshalInput = errors.New("smt2 marshal requires an open AST manager, expression, and transport")
