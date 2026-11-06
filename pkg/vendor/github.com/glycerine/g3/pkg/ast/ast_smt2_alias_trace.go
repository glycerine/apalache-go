package ast

// Source: src/ast/ast_smt2_pp.cpp:708-711; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) traceSMT2Alias(h ASTHandle, name string, depth, weight, level uint32, shared bool) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("smt2_pp") {
		return nil
	}
	return tags.EmitConditional("smt2_pp", "store_result", "src/ast/ast_smt2_pp.cpp", 708,
		func() bool { return true }, func(w io.Writer) (result error) {
			var streamErr error
			write := func(part string) {
				_, err := io.WriteString(w, part)
				if streamErr == nil {
					streamErr = err
				}
			}
			write("a: " + name + " depth: " + strconv.FormatUint(uint64(depth), 10) +
				", weight: " + strconv.FormatUint(uint64(weight), 10) +
				", lvl: " + strconv.FormatUint(uint64(level), 10) + " t: #")
			id, err := m.GetASTID(h)
			if err != nil {
				return err
			}
			write(strconv.FormatUint(uint64(id), 10) + "\n")
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, out.Close()) }()
			if err := m.DisplayLowLevel(out, h, true, true); err != nil {
				return err
			}
			childText, err := out.String()
			if err != nil {
				return err
			}
			sharedBit := "0"
			if shared {
				sharedBit = "1"
			}
			write(childText)
			write(", is-shared: " + sharedBit + "\n")
			return streamErr
		})
}
