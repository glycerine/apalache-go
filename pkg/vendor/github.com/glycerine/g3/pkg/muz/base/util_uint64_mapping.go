package base

import "fmt"

// ParseUint64Mapping ports datalog_parser::parse_map_line through creation of
// the raw name text. Empty/comment-only lines return present=false.
func ParseUint64Mapping(text string) (number uint64, name string, present bool, err error) {
	text = cutDatalogLine(text)
	if len(text) == 0 {
		return 0, "", false, nil
	}
	value, next, ok := ReadUint64(text, 0)
	if !ok {
		return 0, "", false, fmt.Errorf("muz/base: unsigned number expected at byte %d", next)
	}
	if next >= len(text) || text[next] != ' ' {
		return 0, "", false, fmt.Errorf("muz/base: space expected after mapping number")
	}
	return value, text[next+1:], true, nil
}
