package ast

// Source: src/ast/bv_decl_plugin.cpp:801; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) traceBitVectorAllOne(value *numeral.OwnedRational, all bool) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("is_allone") {
		return nil
	}
	return tags.EmitConditional("is_allone", "is_allone", "src/ast/bv_decl_plugin.cpp", 801,
		func() bool { return true }, func(w io.Writer) error {
			spelling, err := m.numerals.String(value)
			if err != nil {
				return err
			}
			result := "0"
			if all {
				result = "1"
			}
			_, err = io.WriteString(w, spelling+" "+result+"\n")
			return err
		})
}
