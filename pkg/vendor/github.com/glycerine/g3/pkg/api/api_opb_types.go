package api

import (
	"time"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type OPBLoadSummary = cmd_context.OPBLoadSummary

type OPBRunOptions struct {
	DisplayStatistics bool
}

type OPBRunResult struct {
	Status   lbool.LBool
	Document opt.OPBDocument
	Summary  OPBLoadSummary
	Elapsed  time.Duration
	Output   string
}
