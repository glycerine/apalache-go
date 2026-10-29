package numeral

import (
	"errors"
)

var ErrNumericOwner = errors.New("mpnum: invalid multivariate polynomial owner")
var ErrNumericAssertion = errors.New("mpnum: multivariate polynomial source assertion")
var ErrNumericPending = errors.New("mpnum: retained polynomial construction")

// These identities are shared by caller adapters and primitive ownership checks.
var ErrNumericCallerOwner = errors.New("mpnum: invalid polynomial owner")
var ErrNumericCallerAssertion = errors.New("mpnum: polynomial source assertion")
var ErrNumericCleanupPending = errors.New("mpnum: retained factorization lexical cleanup")
