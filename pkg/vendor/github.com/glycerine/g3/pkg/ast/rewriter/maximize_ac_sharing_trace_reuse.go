package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:60.

import (
	"io"
	"strconv"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *MaximizeBVSharingConfiguration) traceReuse(i, j int) error {
	return c.emitSharingTrace("reduce_app", 60, func(w io.Writer, _ *uparams.GlobalParamsService) error {
		_, err := io.WriteString(w, "reusing args: "+strconv.Itoa(i)+" "+strconv.Itoa(j)+"\n")
		return err
	})
}
