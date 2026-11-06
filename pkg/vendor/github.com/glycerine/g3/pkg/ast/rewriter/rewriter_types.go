package rewriter

// Source: src/ast/rewriter/rewriter_types.h:29-49.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

type brStatus uint32

const (
	brRewrite1 brStatus = iota
	brRewrite2
	brRewrite3
	brRewriteFull
	brDone
	brFailed
)

const rwUnboundedDepth uint32 = 3

func unsignedToBRStatus(u uint32) brStatus {
	if u >= rwUnboundedDepth {
		return brRewriteFull
	}
	return brStatus(u)
}

// rewriterException retains the source rewriter_exception identity while
// preserving catches of its default_exception base through errors.As.
type rewriterException struct{ z3exception.DefaultException }

func newRewriterException(message string) rewriterException {
	return rewriterException{z3exception.NewDefaultException(message)}
}

// NewRewriterException preserves typed catches in clients implementing source
// rewriter configurations and bit-blaster template checkpoints.
func NewRewriterException(message string) error { return newRewriterException(message) }

func (e rewriterException) Unwrap() error { return e.DefaultException }

// IsRewriterException supports the source's typed catch at tactic boundaries
// without treating AST ownership or parameter errors as rewrite failures.
func IsRewriterException(err error) bool {
	var exception rewriterException
	return errors.As(err, &exception)
}
