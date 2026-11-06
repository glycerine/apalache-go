package transforms

// Source: src/muz/transforms/dl_mk_coi_filter.h prioritized rule-transformer
// plugin surface.

import (
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type ConePlugin struct {
	priority  uint32
	steps     []ConeModelStep
	relations *rel.RelationManager
	closed    bool
	closing   bool
}

func NewConePlugin(priority uint32) *ConePlugin {
	return NewConePluginWithRelations(nil, priority)
}

// NewConePluginWithRelations borrows relations for the source fact-table
// bailout in the bottom-up pass.
func NewConePluginWithRelations(relations *rel.RelationManager, priority uint32) *ConePlugin {
	if priority == 0 {
		priority = 45000
	}
	return &ConePlugin{priority: priority, relations: relations}
}

func (p *ConePlugin) Priority() uint32 { return p.priority }

func (p *ConePlugin) CanDestratifyNegation() bool { return false }

func (p *ConePlugin) Transform(source *base.RuleSet) (*base.RuleSet, error) {
	if p == nil || p.closed || p.closing {
		return nil, base.ErrRuleTransformerOwner
	}
	result, steps, err := coneOfInfluence(source, true, p.relations)
	if err != nil {
		return nil, err
	}
	p.steps = append(p.steps, steps...)
	return result, nil
}

// TakeModelSteps transfers the ordered owning model-converter steps emitted
// by successful transformations. The caller closes them.
func (p *ConePlugin) TakeModelSteps() ([]ConeModelStep, error) {
	if p == nil || p.closed || p.closing {
		return nil, base.ErrRuleTransformerOwner
	}
	result := p.steps
	p.steps = nil
	return result, nil
}

func (p *ConePlugin) Cancel() {}

func (p *ConePlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closing = true
	if err := CloseConeModelSteps(p.steps); err != nil {
		return err
	}
	p.steps = nil
	p.closed = true
	return nil
}
