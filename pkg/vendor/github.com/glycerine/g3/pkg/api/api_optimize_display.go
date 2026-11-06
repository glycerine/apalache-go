package api

// Source: src/api/api_opt.cpp optimize_to_string/statistics/objective display.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/util/statistics"
)

func (o *APIOptimize) WriteObjectives(output io.Writer) error {
	if err := o.open(); err != nil {
		return err
	}
	if output == nil {
		return io.ErrClosedPipe
	}
	return o.command.WriteObjectives(output)
}

func (o *APIOptimize) SMT2() (string, error) {
	if err := o.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	if err := o.command.WriteOptimizationReplaySMT2(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (o *APIOptimize) Statistics() (statistics.Statistics, error) {
	if err := o.open(); err != nil {
		return statistics.Statistics{}, err
	}
	return o.command.Statistics()
}

// StatisticsObject returns an independently owned optimizer statistics snapshot.
func (o *APIOptimize) StatisticsObject() (*APIStatistics, error) {
	value, err := o.Statistics()
	if err != nil {
		return nil, err
	}
	return newAPIStatistics(o.owner, value)
}
