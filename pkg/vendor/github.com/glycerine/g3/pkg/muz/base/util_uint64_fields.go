package base

import "fmt"

func cutDatalogLine(text string) string {
	for index, character := range []byte(text) {
		if character == '#' || character == '\n' || character == '\r' {
			return text[:index]
		}
	}
	return text
}

// ParseUint64Fields ports datalog_parser::parse_rel_line's concrete consumer
// of read_uint64. Empty/comment-only lines return present=false.
func ParseUint64Fields(text string) (values []uint64, present bool, err error) {
	text = cutDatalogLine(text)
	if len(text) == 0 {
		return nil, false, nil
	}
	present = true
	offset := 0
	for {
		for offset < len(text) && text[offset] == ' ' {
			offset++
		}
		if offset == len(text) {
			return values, present, nil
		}
		value, next, ok := ReadUint64(text, offset)
		if !ok {
			return nil, false, fmt.Errorf("muz/base: unsigned number expected at byte %d", next)
		}
		values = append(values, value)
		offset = next
		if offset < len(text) && text[offset] != ' ' {
			return nil, false, fmt.Errorf("muz/base: space expected at byte %d", offset)
		}
	}
}
