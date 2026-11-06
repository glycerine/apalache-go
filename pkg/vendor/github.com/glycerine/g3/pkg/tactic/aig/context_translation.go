package aig

import (
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices copies the source tactic's scalar policy to a target owner.
func (t *Tactic) TranslateWithServices(c *uparams.ParamsContext, cooperation *cooperate.Cooperation) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	next, err := NewTactic(c, nil, cooperation)
	if err != nil {
		return nil, err
	}
	next.pass.DefaultGateEncoding = t.pass.DefaultGateEncoding
	next.pass.PerAssertion = t.pass.PerAssertion
	next.pass.maxMemory = t.pass.maxMemory
	return next, nil
}
