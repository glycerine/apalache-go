package spacer

// Source: src/muz/spacer/spacer_context.h context::display_certificate and
// src/muz/spacer/spacer_context.cpp context::get_proof.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// DisplayCertificate preserves the retained Spacer presentation hook. This
// fork's context implementation deliberately emits no certificate text; the
// fixedpoint interface forwards to this no-op instead of printing the answer
// or inductive model through a different presentation path.
func (c *Context) DisplayCertificate(io.Writer) error {
	return c.open()
}

// Proof preserves context::get_proof: Spacer does not expose an overall proof
// object through the fixedpoint engine, even though its internal SMT checks may
// produce proofs for lemma construction.
func (c *Context) Proof() (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return ast.ASTHandle{}, nil
}

// writeSpacerVerbose follows the live context's verbose_stream insertions;
// minimum zero is an unconditional insertion, without a verbosity read.
func writeSpacerVerbose(minimum uint32, message string) error {
	if minimum != 0 {
		enabled, err := diagnostic.ProcessVerboseEnabled(minimum)
		if err != nil {
			return err
		}
		if !enabled {
			return nil
		}
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return stream.AppendCString(message)
}
