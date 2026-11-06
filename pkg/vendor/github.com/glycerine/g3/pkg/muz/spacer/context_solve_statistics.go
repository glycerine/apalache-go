package spacer

// Source: src/muz/spacer/spacer_context.cpp context::solve statistics
// epilogue.

import (
	"bytes"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (c *Context) printSolveStatistics() error {
	if !c.printStatistics {
		return nil
	}
	collected := statistics.New()
	if err := c.CollectStatistics(collected); err != nil {
		return err
	}
	var rendered bytes.Buffer
	if err := collected.DisplaySMT2(&rendered); err != nil {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return stream.AppendCString(rendered.String())
}
