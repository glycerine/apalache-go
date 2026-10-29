package params

// Source: src/util/params.cpp norm_param_name overloads.

import (
	"strings"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func paramNameByte(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value - 'A' + 'a'
	}
	if value == '-' || value == ':' {
		return '_'
	}
	return value
}

// NormParamName preserves the nullable C-string and single leading colon.
// Only ASCII capitals, hyphen and colon change; UTF-8 bytes are not case-folded.
// The returned string belongs to the caller, not a retained parameter owner.
func NormParamName(text *string) string {
	if text == nil {
		return "_"
	}
	input := *text
	if end := strings.IndexByte(input, 0); end >= 0 {
		input = input[:end]
	}
	if len(input) != 0 && input[0] == ':' {
		input = input[1:]
	}
	if len(input) == 0 {
		return "_"
	}
	var output strings.Builder
	output.Grow(len(input))
	for i := 0; i < len(input); i++ {
		output.WriteByte(paramNameByte(input[i]))
	}
	return output.String()
}

// NormParamSymbolName reads the live owner's bare C-string directly. It does
// not allocate an intermediate spelling string or intern the normalized result.
func NormParamSymbolName(manager *usymbol.Manager, name usymbol.Symbol) (string, error) {
	if manager == nil {
		return "", usymbol.ErrInvalid
	}
	// Source norm_param_name(symbol) reaches bare_str before the C-string
	// overload. A numerical symbol therefore reaches its SASSERT in debug;
	// with that assertion erased or disabled, its empty spelling becomes "_".
	text, present, err := manager.BareString(name)
	if err != nil {
		return "", err
	}
	if !present {
		return "_", nil
	}
	length, start := len(text), 0
	if length != 0 {
		if text[0] == ':' {
			start = 1
		}
	}
	if start == length {
		return "_", nil
	}
	var output strings.Builder
	output.Grow(length - start)
	for i := start; i < length; i++ {
		output.WriteByte(paramNameByte(text[i]))
	}
	return output.String(), nil
}
