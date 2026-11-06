package base

import "strings"

// FileNameWithoutExtension mirrors datalog::get_file_name_without_extension
// from src/muz/base/dl_util.cpp. Both slash styles delimit the basename, and
// a leading dot is part of the name rather than an extension separator.
func FileNameWithoutExtension(name string) string {
	separator := strings.LastIndexAny(name, "\\/")
	base := name[separator+1:]
	dot := strings.LastIndexByte(base, '.')
	if dot > 0 {
		return base[:dot]
	}
	return base
}
