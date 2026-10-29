package smtcore

// Source: src/smt/smt_quantifier.cpp quantifier_manager::imp registry,
// statistics, add_instance, search initialization, and scoped queue storage.

import "github.com/glycerine/g3/pkg/ast"

type SMTQuantifierRegistry struct {
	manager      *ast.ASTManager
	generator    *SMTQuantifierStatGenerator
	fingerprints *SMTFingerprintSet
	schedule     *SMTQIQueueSchedule
	costs        *SMTQICostModel
	statistics   map[ast.ASTHandle]*SMTQuantifierStat
	quantifiers  []ast.ASTHandle
	numInstances uint32
	maxInstances uint32
	scopeLevel   uint32
}

func NewSMTQuantifierRegistry(manager *ast.ASTManager, schedule *SMTQIQueueSchedule, costs *SMTQICostModel, maxInstances uint32) (*SMTQuantifierRegistry, error) {
	if manager == nil || schedule == nil || schedule.manager != manager || costs == nil || costs.manager != manager {
		return nil, ast.ErrASTHandle
	}
	generator, err := NewSMTQuantifierStatGenerator(manager)
	if err != nil {
		return nil, err
	}
	return &SMTQuantifierRegistry{
		manager: manager, generator: generator, fingerprints: NewSMTFingerprintSet(), schedule: schedule, costs: costs,
		statistics: make(map[ast.ASTHandle]*SMTQuantifierStat), maxInstances: maxInstances,
	}, nil
}

func (r *SMTQuantifierRegistry) Add(quantifier ast.ASTHandle, generation uint32) error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if _, found := r.statistics[quantifier]; found {
		return ast.ErrASTDomain
	}
	stat, err := r.generator.Generate(quantifier, generation)
	if err != nil {
		return err
	}
	r.statistics[quantifier] = stat
	r.quantifiers = append(r.quantifiers, quantifier)
	return nil
}

func (r *SMTQuantifierRegistry) Delete(quantifier ast.ASTHandle) error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if len(r.quantifiers) == 0 || r.quantifiers[len(r.quantifiers)-1] != quantifier {
		return ast.ErrASTDomain
	}
	delete(r.statistics, quantifier)
	r.quantifiers = r.quantifiers[:len(r.quantifiers)-1]
	return nil
}

func (r *SMTQuantifierRegistry) Empty() bool { return r == nil || len(r.quantifiers) == 0 }
func (r *SMTQuantifierRegistry) NumQuantifiers() uint32 {
	if r == nil {
		return 0
	}
	return uint32(len(r.quantifiers))
}
func (r *SMTQuantifierRegistry) Quantifiers() []ast.ASTHandle {
	if r == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), r.quantifiers...)
}

func (r *SMTQuantifierRegistry) Stat(quantifier ast.ASTHandle) (*SMTQuantifierStat, error) {
	if r == nil || r.manager == nil {
		return nil, ast.ErrASTHandle
	}
	stat := r.statistics[quantifier]
	if stat == nil {
		return nil, ast.ErrASTDomain
	}
	return stat, nil
}

func (r *SMTQuantifierRegistry) Generation(quantifier ast.ASTHandle) (uint32, error) {
	stat, err := r.Stat(quantifier)
	if err != nil {
		return 0, err
	}
	return stat.Generation(), nil
}

func (r *SMTQuantifierRegistry) AddInstance(quantifier, pattern ast.ASTHandle, bindings []*SMTEnode, maxGeneration, minTopGeneration, maxTopGeneration uint32) (bool, error) {
	if r == nil || r.manager == nil || r.fingerprints == nil || r.schedule == nil || r.costs == nil {
		return false, ast.ErrASTHandle
	}
	stat, err := r.Stat(quantifier)
	if err != nil {
		return false, err
	}
	// Preserve quantifier_manager's public forwarding behavior: the supplied
	// max_top_generation is replaced with the original max_generation.
	_ = maxTopGeneration
	sourceMaxTopGeneration := maxGeneration
	if stat.Generation() > maxGeneration {
		maxGeneration = stat.Generation()
	}
	if r.numInstances > r.maxInstances {
		return false, nil
	}
	stat.UpdateMaxGeneration(maxGeneration)
	id, err := r.manager.GetASTID(quantifier)
	if err != nil {
		return false, err
	}
	fingerprint, err := r.fingerprints.Insert(quantifier, id, bindings)
	if err != nil || fingerprint == nil {
		return false, err
	}
	cost, err := r.costs.Cost(quantifier, pattern, stat, maxGeneration, minTopGeneration, sourceMaxTopGeneration, r.scopeLevel)
	if err != nil {
		return false, err
	}
	if err = r.schedule.Insert(fingerprint, cost, maxGeneration); err != nil {
		return false, err
	}
	r.numInstances++
	return true, nil
}

func (r *SMTQuantifierRegistry) InitSearch() error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	r.numInstances = 0
	for _, quantifier := range r.quantifiers {
		r.statistics[quantifier].ResetInstancesCurrentSearch()
	}
	return nil
}

func (r *SMTQuantifierRegistry) PushScope() error {
	if r == nil || r.fingerprints == nil || r.schedule == nil {
		return ast.ErrASTHandle
	}
	if err := r.schedule.PushScope(); err != nil {
		return err
	}
	if err := r.fingerprints.PushScope(); err != nil {
		_ = r.schedule.PopScopes(1)
		return err
	}
	r.scopeLevel++
	return nil
}

func (r *SMTQuantifierRegistry) PopScopes(count uint32) error {
	if r == nil || r.fingerprints == nil || r.schedule == nil {
		return ast.ErrASTHandle
	}
	if count == 0 || count > r.scopeLevel {
		return ast.ErrASTDomain
	}
	if err := r.schedule.PopScopes(count); err != nil {
		return err
	}
	if err := r.fingerprints.PopScopes(count); err != nil {
		return err
	}
	r.scopeLevel -= count
	return nil
}

func (r *SMTQuantifierRegistry) Reset() error {
	if r == nil {
		return nil
	}
	if err := r.schedule.Reset(); err != nil {
		return err
	}
	r.fingerprints.Reset()
	r.statistics = make(map[ast.ASTHandle]*SMTQuantifierStat)
	r.quantifiers = nil
	r.numInstances = 0
	r.scopeLevel = 0
	return nil
}
