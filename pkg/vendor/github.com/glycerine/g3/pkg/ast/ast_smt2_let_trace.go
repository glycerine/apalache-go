package ast

// Source: src/ast/ast_smt2_pp.cpp:806,818,833; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) traceSMT2Let(line int, label string, value int) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pp_let") {
		return nil
	}
	body := label + strconv.Itoa(value) + "\n"
	return tags.EmitConditional("pp_let", "pp_let", "src/ast/ast_smt2_pp.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, body)
			return err
		})
}
