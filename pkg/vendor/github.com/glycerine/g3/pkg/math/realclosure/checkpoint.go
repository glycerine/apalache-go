package realclosure

import (
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrRealClosureCanceled = z3exception.NewDefaultException(messages.Canceled)
