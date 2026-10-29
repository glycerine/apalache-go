package transforms

// Source: src/muz/transforms/dl_mk_coi_filter.h prioritized rule-transformer
// plugin surface.

import (
	"github.com/glycerine/g3/pkg/muz/base"
)

type ConePlugin struct {
	priority uint32
	closed   bool
}

func NewConePlugin(priority uint32) *ConePlugin {
	if priority == 0 {
		priority = 45000
	}
	return &ConePlugin{priority: priority}
}

func (p *ConePlugin) Priority() uint32 { return p.priority }

func (p *ConePlugin) CanDestratifyNegation() bool { return false }

func (p *ConePlugin) Transform(source *base.RuleSet) (*base.RuleSet, error) {
	if p == nil || p.closed {
		return nil, base.ErrRuleTransformerOwner
	}
	return ConeOfInfluence(source)
}

func (p *ConePlugin) Cancel() {}

func (p *ConePlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	return nil
}
