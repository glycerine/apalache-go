package fp

import "errors"

// ErrUnstratifiedNegation is the source context::close query exception.
var ErrUnstratifiedNegation = errors.New("muz/fp: negation is not stratified")
