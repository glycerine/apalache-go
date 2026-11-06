package sls

// Source: src/tactic/sls/sls_engine.cpp:552,582,594.

import (
	"time"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (e *Engine) reportSearchSummary() error {
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.AppendCString("(restarts: "); err != nil {
		return err
	}
	if err = stream.AppendUint32(e.stats.Restarts); err != nil {
		return err
	}
	if err = stream.AppendCString(" flips: "); err != nil {
		return err
	}
	if err = stream.AppendUint32(e.stats.Moves); err != nil {
		return err
	}
	if err = stream.AppendCString(" fps: "); err != nil {
		return err
	}
	seconds := time.Since(e.stats.Started).Seconds()
	if err = stream.AppendFloat64(float64(e.stats.Moves) / seconds); err != nil {
		return err
	}
	if err = stream.AppendCString(")\n"); err != nil {
		return err
	}
	return stream.Flush()
}
