package ast

// Source symbol interning byte-input boundary used by actual AST name buffers.

import (
	"bytes"
	"strconv"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// The source string_buffer<64> is temporary name scratch. A Go bytes.Buffer
// retains the borrowed symbol spelling and final C-string interning order.
func (m *ASTManager) modelValueName(sortName usymbol.Symbol, index int32) (usymbol.Symbol, error) {
	var initial [64]byte
	buffer := bytes.NewBuffer(initial[:0])
	view, e := uparams.SymbolCStringView(m.Symbols, sortName)
	if e == nil && (sortName.IsNumerical() || sortName.IsNull()) {
		e = ErrASTDomain
	}
	if e == nil {
		var n uint64
		n, e = view.Length()
		if e == nil {
			for i := uint64(0); i < n; i++ {
				var v byte
				v, e = view.Byte(i)
				if e != nil {
					break
				}
				buffer.WriteByte(v)
			}
		}
	}

	if e != nil {
		return usymbol.Symbol{}, e
	}
	buffer.WriteString("!val!")
	var digits [24]byte
	number := strconv.AppendInt(digits[:0], int64(index), 10)
	buffer.Write(number)
	buffer.WriteByte(0)
	return m.Symbols.InternBytes(buffer.Bytes())
}
