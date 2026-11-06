package smtcore

// Source: src/smt/theory_str.cpp gen_val_string and get_next_val_encode.

import "github.com/glycerine/g3/pkg/ast"

// Retained value-option producers supply one alphabet digit per byte and a
// final zero carry sentinel. The first digit selects the rightmost byte.
func (t *SMTStringTheory) stringValueFromEncoding(length int32, encoding []int32) (string, error) {
	size := int32(len(t.charsetLookup))
	if size <= 0 || size >= int32(len(t.charset)) || length < 0 || int64(len(encoding)) != int64(length)+1 {
		return "", ast.ErrASTDomain
	}
	if encoding[len(encoding)-1] != 0 {
		return "", ast.ErrASTDomain
	}
	value := make([]byte, int(length))
	for i := range value {
		value[i] = t.charset[0]
	}
	for index := 0; index < len(encoding)-1; index++ {
		digit := encoding[index]
		if digit < 0 || digit >= size {
			return "", ast.ErrASTDomain
		}
		value[int(length)-1-index] = t.charset[digit]
	}
	// The active source alphabet excludes NUL, so its c_str conversion never
	// truncates a retained generated word. Go directly owns the byte string.
	return string(value), nil
}

func (t *SMTStringTheory) nextStringValueEncoding(base []int32) (next []int32, covered bool, result error) {
	size := int32(len(t.charsetLookup))
	if size <= 0 || size >= int32(len(t.charset)) || len(base) == 0 || base[len(base)-1] != 0 {
		return nil, false, ast.ErrASTDomain
	}
	next = make([]int32, len(base))
	carry := int32(1)
	for index, digit := range base {
		if digit < 0 || digit >= size {
			return nil, false, ast.ErrASTDomain
		}
		value := digit + carry
		carry = value / size
		next[index] = value % size
	}
	if next[len(next)-1] > 0 {
		return nil, true, nil
	}
	return next, false, nil
}
