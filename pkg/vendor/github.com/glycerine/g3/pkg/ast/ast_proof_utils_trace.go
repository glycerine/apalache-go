package ast

// Source: src/ast/proofs/proof_utils.cpp:993-996; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// TraceProofUtilsQuantifierMismatch emits the retained proof_utils diagnostic
// without exposing the manager's borrowed trace service to child packages.
func (m *ASTManager) TraceProofUtilsQuantifierMismatch(q ASTHandle, substitutionSize uint32) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("proof_utils") {
		return nil
	}
	return tags.EmitConditional("proof_utils", "instantiate", "src/ast/proofs/proof_utils.cpp", 993,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "quantifier has different number of variables than substitution"); err != nil {
				return err
			}
			c, p, g, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(q, c, p, g)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body+"\n"+strconv.FormatUint(uint64(substitutionSize), 10)+"\n")
			return err
		})
}
