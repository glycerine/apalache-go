package control

import "errors"

var (
	ErrPrecondition = errors.New("g3: control source precondition")
	ErrOwner        = errors.New("g3: control owner or lifetime mismatch")
)
