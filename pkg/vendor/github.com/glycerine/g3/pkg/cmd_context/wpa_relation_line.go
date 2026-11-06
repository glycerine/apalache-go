package cmd_context

import "github.com/glycerine/g3/pkg/muz/base"

func parseWPARelationLine(line string) ([]uint64, bool, error) {
	return base.ParseUint64Fields(line)
}
