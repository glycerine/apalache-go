package cmd_context

import (
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
)

func parseWPAMapLine(line string) (uint64, string, bool, error) {
	return parseWPAMapLineWithNames(line, true)
}

func parseWPAMapLineWithNames(line string, useNames bool) (uint64, string, bool, error) {
	number, name, present, err := base.ParseUint64Mapping(line)
	if err != nil || !present {
		return 0, "", present, err
	}
	if !useNames {
		return number, "<names ignored>", true, nil
	}
	for _, marker := range []string{" SC_EXTERN ", " _ZONE_"} {
		if cut := strings.Index(name, marker); cut >= 0 {
			name = name[:cut]
		}
	}
	if len(name) > len("Constant ") && strings.HasSuffix(name, "Constant ") {
		name = strings.TrimSuffix(name, "Constant ")
	}
	if name == "" {
		// Source indexes the final byte here. Return an explicit error for
		// that undefined empty-name input instead of indexing before zero.
		return 0, "", false, fmt.Errorf("empty symbol name")
	}
	name = strings.TrimSuffix(name, " ")
	return number, name, true, nil
}
