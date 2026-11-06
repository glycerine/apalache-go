package interp

// Source: src/interp/iz3exception.h.

import "github.com/glycerine/g3/pkg/util/z3exception"

type Error struct{ message string }

func NewError(message string) Error              { return Error{message: message} }
func (e Error) Error() string                    { return e.message }
func (e Error) Message() string                  { return e.message }
func (e Error) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e Error) HasErrorCode() bool               { return false }
