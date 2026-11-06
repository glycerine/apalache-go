package params

// Source: src/api/api_solver.cpp Z3_solver_translate carries params_ref across
// AST managers. Go parameter services own their symbols and numerals, so this
// operation copies each concrete parameter into the target service.

import (
	"errors"
	"math"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type translatedParamSymbol struct {
	number           uint32
	text             string
	numeric, present bool
}
type translatedParamEntry struct {
	key, symbol translatedParamSymbol
	kind        ParamKind
	bits        uint64
	text        string
	present     bool
}

func (c *ParamsContext) snapshotTranslationSymbol(s usymbol.Symbol) (translatedParamSymbol, error) {
	if s.IsNumerical() {
		n, err := s.Number()
		return translatedParamSymbol{number: n, numeric: true, present: true}, err
	}
	text, present, err := c.SymbolsField.BareString(s)
	return translatedParamSymbol{text: text, present: present}, err
}
func (c *ParamsContext) translationEntries(v *ParamsRef) ([]translatedParamEntry, error) {
	if c == nil {
		return nil, ErrParamsOwner
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.check(v); err != nil {
		return nil, err
	}
	if v.Slot == 0 {
		return nil, nil
	}
	block := &c.Blocks[v.Slot-1]
	result := make([]translatedParamEntry, 0, block.Size)
	for i := uint32(0); i < block.Size; i++ {
		entry := c.Entry(block, i)
		key, err := c.snapshotTranslationSymbol(entry.key)
		if err != nil {
			return nil, err
		}
		out := translatedParamEntry{key: key, kind: entry.Kind, bits: entry.bits}
		switch entry.Kind {
		case ParamBool, ParamUInt, ParamDouble:
		case ParamSymbol:
			out.symbol, err = c.snapshotTranslationSymbol(entry.symbol)
		case ParamString:
			out.present = entry.text.present
			if entry.text.present {
				if entry.text.borrowed {
					out.text, _, err = c.SymbolsField.BareString(entry.symbol)
				} else {
					out.text, err = entry.text.owned.read()
				}
			}
		case ParamNumeral:
			out.text, err = c.NumeralsField.String(&entry.Rat)
		default:
			return nil, ErrParamsPrecondition
		}
		if err != nil {
			return nil, err
		}
		result = append(result, out)
	}
	return result, nil
}
func (s translatedParamSymbol) into(manager *usymbol.Manager) (usymbol.Symbol, error) {
	if s.numeric {
		return usymbol.Numerical(s.number), nil
	}
	if !s.present {
		return usymbol.Symbol{}, nil
	}
	return manager.Intern(s.text)
}

// TranslateTo creates an independently owned parameter value in target. On
// failure all target resources acquired by the operation are released.
func (c *ParamsContext) TranslateTo(value *ParamsRef, target *ParamsContext) (result ParamsRef, resultErr error) {
	entries, err := c.translationEntries(value)
	if err != nil {
		return result, err
	}
	if target == nil {
		return result, ErrParamsOwner
	}
	result, err = target.NewValue()
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, target.Del(&result))
			result = ParamsRef{}
		}
	}()
	for _, entry := range entries {
		key, e := entry.key.into(target.SymbolsField)
		if e != nil {
			return result, e
		}
		switch entry.kind {
		case ParamBool:
			e = target.SetBool(&result, key, entry.bits != 0)
		case ParamUInt:
			e = target.SetUInt(&result, key, uint32(entry.bits))
		case ParamDouble:
			e = target.SetDouble(&result, key, math.Float64frombits(entry.bits))
		case ParamSymbol:
			var symbol usymbol.Symbol
			symbol, e = entry.symbol.into(target.SymbolsField)
			if e == nil {
				e = target.SetSymbol(&result, key, symbol)
			}
		case ParamString:
			var text *string
			if entry.present {
				text = &entry.text
			}
			e = target.SetString(&result, key, text)
		case ParamNumeral:
			rational, makeErr := target.NumeralsField.NewValue()
			e = makeErr
			if e == nil {
				e = target.NumeralsField.SetString(&rational, entry.text)
				if e == nil {
					e = target.SetRat(&result, key, &rational)
				}
				e = errors.Join(e, target.NumeralsField.Del(&rational))
			}
		}
		if e != nil {
			return result, e
		}
	}
	return result, nil
}
