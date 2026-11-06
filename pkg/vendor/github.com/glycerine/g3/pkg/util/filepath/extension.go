package filepath

// Source: src/util/file_path.h:23-35.

import "strings"

// Extension returns the suffix after the last dot in name. The bool reports
// whether a dot was present, so a trailing dot is distinguishable from no
// extension. A NUL ends the visible name, matching the source API boundary.
func Extension(name string) (extension string, found bool) {
	if end := strings.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	dot := strings.LastIndexByte(name, '.')
	if dot < 0 {
		return "", false
	}
	return name[dot+1:], true
}
