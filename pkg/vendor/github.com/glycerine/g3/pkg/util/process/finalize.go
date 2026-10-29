package process

// Source: src/api/dll/mem_initializer.cpp, src/shell/mem_initializer.cpp,
// src/test/mem_initializer.cpp, generated mem_finalize body.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/prime"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// FinalizeGenerated runs the six concrete source finalizers in generated
// order. All services are borrowed and remain owned by the caller. An error
// stops at the exact child; the memory guard remains finalizing, matching the
// source's absence of a catch or continuation after a failed destructor.
func FinalizeGenerated(p *memory.ProcessAccounting, d *diagnostic.DebugContext, g *params.GlobalParamsService, r *numeral.RationalGlobalService, s *symbol.Service, tr *diagnostic.TraceTags) error {
	if p == nil || !p.Valid() {
		return arena.ErrConfig
	}
	if !p.InitializationStarted() {
		_, err := p.BeginFinalize()
		return err
	}
	if d == nil || g == nil || r == nil || s == nil || tr == nil {
		return arena.ErrConfig
	}
	bound := currentGenerated
	if bound == nil || bound.closed || bound.process != p || bound.debug != d || bound.Globals != g || bound.rational != r || bound.symbols != s || bound.Trace != tr ||
		diagnostic.BorrowProcessDebugContext() != d || params.BorrowProcessGlobalParamsService() != g {
		return arena.ErrConfig
	}
	active, err := p.BeginFinalize()
	if err != nil || !active {
		return err
	}
	if err = d.FinalizeDebug(); err != nil {
		return err
	}
	if err = g.FinalizeSource(); err != nil {
		return err
	}
	if err = prime.FinalizeProcessGenerator(); err != nil {
		return err
	}
	if err = r.FinalizeSource(); err != nil {
		return err
	}
	if err = s.FinalizeSource(); err != nil {
		return err
	}
	tr.Finalize()
	return p.CompleteFinalize()
}
