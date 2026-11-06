package api

import (
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"time"
)

type WCNFLoadSummary = cmd_context.WCNFLoadSummary
type WCNFRunOptions struct{ DisplayStatistics bool }
type WCNFRunResult struct {
	Status   lbool.LBool
	Document opt.WCNFDocument
	Summary  WCNFLoadSummary
	Elapsed  time.Duration
	Output   string
}
