package api

import lp "github.com/glycerine/g3/pkg/util/lp"

type MPSRunOptions struct {
	Minimum         bool
	PrintStatistics bool
	ReportFrequency uint32
}

type MPSRunResult struct {
	Status    lp.Status
	Objective float64
	Columns   map[string]float64
	Output    string
}
