package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:76.

import (
	"io"
	"strconv"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *MaximizeBVSharingConfiguration) traceTreeLoop(count int) error {
	return c.emitSharingTrace("reduce_app", 76, func(w io.Writer, _ *uparams.GlobalParamsService) error {
		_, err := io.WriteString(w, "tree-loop: num_args: "+strconv.Itoa(count)+"\n")
		return err
	})
}
