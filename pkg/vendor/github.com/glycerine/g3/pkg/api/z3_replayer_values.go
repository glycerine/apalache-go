package api

import "fmt"

type z3ReplayKind uint8

const (
	replayInt64 z3ReplayKind = iota
	replayUint64
	replayDouble
	replayString
	replaySymbol
	replayObject
	replayUintArray
	replayIntArray
	replaySymbolArray
	replayObjectArray
	replayFloat
)

type z3ReplayValue struct {
	kind          z3ReplayKind
	i             int64
	u             uint64
	d             float64
	f             float32
	s             string
	nullString    bool
	nullSymbol    bool
	numericSymbol bool
}

func (r *Z3Replayer) value(position uint32, kind z3ReplayKind) (z3ReplayValue, error) {
	if r == nil || r.closed || int(position) >= len(r.args) {
		return z3ReplayValue{}, ErrZ3ReplayReference
	}
	value := r.args[position]
	if value.kind != kind {
		return z3ReplayValue{}, fmt.Errorf("%w: expecting %s at position %d but got %s", ErrZ3ReplayReference, kind, position, value.kind)
	}
	return value, nil
}

func (k z3ReplayKind) String() string {
	names := [...]string{"int64", "uint64", "double", "string", "symbol", "object", "uint_array", "int_array", "symbol_array", "object_array", "float"}
	if int(k) >= len(names) {
		return "unknown"
	}
	return names[k]
}

func (r *Z3Replayer) Int64(p uint32) (int64, error) { v, e := r.value(p, replayInt64); return v.i, e }
func (r *Z3Replayer) Int(p uint32) (int32, error)   { v, e := r.Int64(p); return int32(v), e }
func (r *Z3Replayer) Uint64(p uint32) (uint64, error) {
	v, e := r.value(p, replayUint64)
	return v.u, e
}
func (r *Z3Replayer) Uint(p uint32) (uint32, error)   { v, e := r.Uint64(p); return uint32(v), e }
func (r *Z3Replayer) Bool(p uint32) (bool, error)     { v, e := r.Int64(p); return v != 0, e }
func (r *Z3Replayer) Float(p uint32) (float32, error) { v, e := r.value(p, replayFloat); return v.f, e }
func (r *Z3Replayer) Double(p uint32) (float64, error) {
	v, e := r.value(p, replayDouble)
	return v.d, e
}
func (r *Z3Replayer) String(p uint32) (string, error) {
	v, e := r.value(p, replayString)
	return v.s, e
}
func (r *Z3Replayer) Symbol(p uint32) (string, error) {
	v, e := r.value(p, replaySymbol)
	return v.s, e
}
func (r *Z3Replayer) Object(p uint32) (uint64, error) {
	v, e := r.value(p, replayObject)
	return v.u, e
}

func (r *Z3Replayer) SetInt64(p uint32, value int64) error {
	if _, err := r.value(p, replayInt64); err != nil {
		return err
	}
	r.args[p].i = value
	return nil
}
func (r *Z3Replayer) SetUint64(p uint32, value uint64) error {
	if _, err := r.value(p, replayUint64); err != nil {
		return err
	}
	r.args[p].u = value
	return nil
}
func (r *Z3Replayer) SetObject(p uint32, value uint64) error {
	if _, err := r.value(p, replayObject); err != nil {
		return err
	}
	r.args[p].u = value
	return nil
}

// SetString updates a string output argument as get_str_addr does in C++.
func (r *Z3Replayer) SetString(p uint32, value string) error {
	if _, err := r.value(p, replayString); err != nil {
		return err
	}
	r.args[p].s = value
	r.args[p].nullString = false
	return nil
}

// StringIsNull distinguishes a null string output from an empty string.
func (r *Z3Replayer) StringIsNull(p uint32) (bool, error) {
	value, err := r.value(p, replayString)
	return value.nullString, err
}

func (r *Z3Replayer) setStringNull(p uint32) error {
	if _, err := r.value(p, replayString); err != nil {
		return err
	}
	r.args[p].s = ""
	r.args[p].nullString = true
	return nil
}
