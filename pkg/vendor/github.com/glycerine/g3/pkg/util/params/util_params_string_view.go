package params

// Source: src/util/params.cpp:639-648,902-930; src/util/symbol.h:96;
// scripts/mk_genfile_common.py:895-939.

import (
	"math"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Cold borrowed source pointer. Captured storage/provider/ref comes from the
// actual returned entry; no context workspace, copied string or RC is acquired.
type ParamsStringView struct {
	owned        *paramsOwnedString
	context      *ParamsContext
	symbols      *usymbol.Manager
	symbol       usymbol.Symbol
	literal      string
	Mode         uint8
	PresentField bool
}

func LiteralParamsStringView(s string) ParamsStringView {
	return ParamsStringView{literal: s, Mode: 1, PresentField: true}
}
func (v ParamsStringView) Present() bool { return v.PresentField }
func (v ParamsStringView) Byte(i uint64) (byte, error) {
	if !v.PresentField {
		return 0, utext.ErrNullCString
	}
	switch v.Mode {
	case 1:
		if i >= uint64(len(v.literal)) {
			if i == uint64(len(v.literal)) {
				return 0, nil
			}
			return 0, arena.ErrBounds
		}
		return v.literal[i], nil
	case 2:
		if v.context == nil {
			return 0, ErrParamsOwner
		}
		if v.context.Closed {
			return 0, arena.ErrClosed
		}
		value, e := v.owned.read()
		if e != nil {
			return 0, e
		}
		if i > uint64(len(value)) {
			return 0, arena.ErrBounds
		}
		if i == uint64(len(value)) {
			return 0, nil
		}
		return value[i], nil
	case 3:
		if v.symbols == nil {
			return 0, ErrParamsOwner
		}
		text, present, e := v.symbols.BareString(v.symbol)
		if e != nil {
			return 0, e
		}
		if !present {
			return 0, arena.ErrInvalidSlot
		}
		if i > uint64(len(text)) {
			return 0, arena.ErrBounds
		}
		if i == uint64(len(text)) {
			return 0, nil
		}
		return text[i], nil
	}
	return 0, ErrParamsOwner
}
func (v ParamsStringView) Length() (uint64, error) {
	for i := uint64(0); ; i++ {
		b, e := v.Byte(i)
		if e != nil {
			return 0, e
		}
		if b == 0 {
			return i, nil
		}
		if i == math.MaxUint64 {
			return 0, arena.ErrBounds
		}
	}
}
func (c *ParamsContext) GetStringViewFallbackName(v *ParamsRef, k *string, f *ParamsRef, d *string) (ParamsStringView, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, usymbol.Symbol{}, k, true, ParamString, f)
	if e != nil {
		return ParamsStringView{}, e
	}
	if !ok {
		if d == nil {
			return ParamsStringView{}, nil
		}
		return LiteralParamsStringView(*d), nil
	}
	if !x.text.present {
		return ParamsStringView{}, nil
	}
	if x.text.borrowed {
		return ParamsStringView{symbols: c.SymbolsField, symbol: x.symbol, Mode: 3, PresentField: true}, nil
	}
	return ParamsStringView{owned: x.text.owned, context: c, Mode: 2, PresentField: true}, nil
}

// String copies the reached C-string bytes into an independent native value.
func (v ParamsStringView) String() (string, error) {
	if v.Mode == 1 && v.PresentField {
		text := v.literal
		if i := strings.IndexByte(text, 0); i >= 0 {
			text = text[:i]
		}
		return text, nil
	}
	var text strings.Builder
	for i := uint64(0); ; i++ {
		ch, e := v.Byte(i)
		if e != nil {
			return "", e
		}
		if ch == 0 {
			return text.String(), nil
		}
		if i >= uint64(^uint(0)>>1)-1 {
			return "", arena.ErrBounds
		}
		text.WriteByte(ch)
	}
}
