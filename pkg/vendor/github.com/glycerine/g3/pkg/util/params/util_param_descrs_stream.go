package params

import (
	"bytes"
	"errors"
	"io"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DisplayInto emits source param_descrs::display into a borrowed byte buffer.
// Error keeps the prefix emitted before it.
func (d *ParamDescrs) DisplayInto(dst *bytes.Buffer, indent uint32, smt2, includeDescr bool) error {
	if dst == nil {
		return ErrParamDescrsPrecondition
	}
	return d.displayInto(dst, indent, smt2, includeDescr)
}

// DisplayTo writes directly to the borrowed output stream. A refusing stream
// enters sticky failbit while the source descriptor traversal continues.
func (d *ParamDescrs) DisplayTo(dst io.Writer, indent uint32, smt2, includeDescr bool) error {
	if dst == nil {
		return ErrParamDescrsPrecondition
	}
	w := &sourceTextWriter{dst: dst}
	return errors.Join(d.displayInto(w, indent, smt2, includeDescr), w.failure)
}

func (d *ParamDescrs) displayInto(dst sourceTextBuffer, indent uint32, smt2, includeDescr bool) error {
	if d == nil {
		return ErrParamDescrsPrecondition
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.open(); e != nil {
		return e
	}
	var names []usymbol.Symbol
	for key := range d.entries.All() {
		names = append(names, key)
	}
	if e := d.sortSymbols(names); e != nil {
		return e
	}
	for _, key := range names {
		for j := uint32(0); j < indent; j++ {
			dst.WriteByte(' ')
		}
		if smt2 {
			dst.WriteByte(':')
		}
		if e := d.writeNameInto(dst, key, smt2); e != nil {
			return e
		}
		info, found, e := d.find(key)
		if e != nil {
			return e
		}
		if !found {
			return ErrParamDescrsPrecondition
		}
		if e := sourceAssertParamDescription(info.descr.present); e != nil {
			return e
		}
		dst.WriteString(" (")
		dst.WriteString(info.kind.String())
		dst.WriteByte(')')
		if includeDescr {
			dst.WriteByte(' ')
			// Null CString sets source ostream failbit at this actual output action.
			// This byte-destination adapter returns a typed failure with that prefix.
			if !info.descr.present {
				return utext.ErrNullCString
			}
			dst.WriteString(info.descr.value)
		}
		if info.def.present {
			dst.WriteString(" (default: ")
			dst.WriteString(info.def.value)
			dst.WriteByte(')')
		}
		dst.WriteByte('\n')
	}
	return nil
}
func (d *ParamDescrs) writeNameInto(dst sourceTextBuffer, k usymbol.Symbol, smt2 bool) error {
	m := d.manager
	if e := m.Validate(k); e != nil {
		return e
	}
	if k.Value == 0 || k.IsNumerical() {
		return ErrParamDescrsPrecondition
	}
	text, _, e := m.BareString(k)
	if e != nil {
		return e
	}
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if smt2 && ch == '_' {
			ch = '-'
		} else if !smt2 && ch == '-' {
			ch = '_'
		} else if ch >= 'A' && ch <= 'Z' {
			ch = ch - 'A' + 'a'
		}
		dst.WriteByte(ch)
	}
	return nil
}
