package api

import (
	"github.com/glycerine/g3/pkg/shell"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type DatalogRelationResult struct {
	Name   string
	Status lbool.LBool
	Facts  [][]uint64
}

type DatalogRunResult struct {
	Relations []DatalogRelationResult
	Timings   shell.DatalogFrontendTimings
	Output    string
}
