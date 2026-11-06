package ast

// Source: src/ast/format.cpp:147-152; native caller buffers.

import (
	"bytes"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// FormatBuffer interns the caller's current native bytes before constructing
// the format node. Symbol interning owns its immutable copy.
func (m *ASTManager) FormatBuffer(b *bytes.Buffer) (ASTHandle, error) {
	if b == nil {
		return ASTHandle{}, ErrASTHandle
	}
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	name, e := f.Symbols.InternBytes(b.Bytes())
	if e != nil {
		return ASTHandle{}, e
	}
	p := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
	r, e := f.formatApp(ASTFormatString, &p, nil)
	return f.external(r), e
}

// A retained numerical formatting result is borrowed until its source
// temporary ends. Copy its checked bytes directly into native output storage.
func appendASTFormattedCString(b *bytes.Buffer, text unumeral.ArbitraryFloatFormattedCString) error {
	n, e := text.Length()
	if e != nil {
		return e
	}
	if n > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	for i := uint64(0); i < n; i++ {
		c, e := text.Byte(i)
		if e != nil {
			return e
		}
		b.WriteByte(c)
	}
	return nil
}
