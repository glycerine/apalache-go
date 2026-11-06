package rel

// Source: src/muz/rel/dl_product_relation.cpp aligned_union_fn and the
// external/interval concrete union and widening operations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalUnionOwner struct {
	runtime *memory.Runtime
	nested  memory.ConstructionFailure
	fresh   *ExternalRelation
}

func (o *externalIntervalUnionOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.fresh != nil {
		if err := o.fresh.Close(); err != nil {
			return err
		}
		o.fresh = nil
	}
	return nil
}

type externalIntervalUnionCleanupFailure struct {
	owner             *externalIntervalUnionOwner
	original, cleanup error
	reported          error
	text              string
	committed         bool
}

var _ memory.ConstructionFailure = (*externalIntervalUnionCleanupFailure)(nil)

func (f *externalIntervalUnionCleanupFailure) Error() string        { return f.text }
func (f *externalIntervalUnionCleanupFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalUnionCleanupFailure) OriginalCause() error { return f.original }
func (f *externalIntervalUnionCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalUnionCleanupFailure) assignmentCommitted() bool {
	return f.committed
}
func (f *externalIntervalUnionCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalUnionCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalIntervalUnion(owner *externalIntervalUnionOwner, cause error) error {
	committed := cause == nil || externalAssignmentWasCommitted(cause)
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalUnionCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(), committed: committed,
	}
}

func applyExternalIntervalAssignment(destination **IntervalRelation, candidate *IntervalRelation, assignment func() error) error {
	cause := assignment()
	if cause == nil || externalAssignmentWasCommitted(cause) {
		*destination = candidate
	}
	return cause
}

func (p *ExternalIntervalProduct) unionCompatible(other *ExternalIntervalProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.plugin != other.plugin || len(p.sorts) != len(other.sorts) || len(p.kinds) != len(other.kinds) {
		return ErrTableRelation
	}
	for i := range p.sorts {
		if p.sorts[i] != other.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	if !externalIntervalProductKinds(p.kinds) || !externalIntervalProductKinds(other.kinds) {
		return ErrTableRelation
	}
	return nil
}

func externalIntervalProductKindsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (p *ExternalIntervalProduct) canonicalizeUnalignedUnion(source, delta *ExternalIntervalProduct) {
	unaligned := !externalIntervalProductKindsEqual(p.kinds, source.kinds)
	if delta != nil {
		unaligned = unaligned || !externalIntervalProductKindsEqual(p.kinds, delta.kinds)
	}
	if !unaligned {
		return
	}
	// get_common_spec orders the exact live pair by its registered family
	// identifiers. The concrete Go pair uses the matching external-first
	// canonical representation while its typed components need no conversion.
	p.kinds = []string{"external_relation", "interval_relation"}
	if delta != nil {
		delta.kinds = []string{"external_relation", "interval_relation"}
	}
}

// Union performs the source aligned update after converting an unaligned exact
// pair to its common specification. External and interval plugins do not
// support cross-kind unions in both directions, so only the two corresponding
// component unions are applied. Local interval candidates are published after
// the external assignment commits, even when only cleanup of that committed
// callback remains pending.
func (p *ExternalIntervalProduct) Union(source, delta *ExternalIntervalProduct, widen bool) (resultErr error) {
	if err := p.unionCompatible(source); err != nil {
		return err
	}
	if delta != nil {
		if delta == p || delta == source {
			return ast.ErrASTDomain
		}
		if err := p.unionCompatible(delta); err != nil {
			return err
		}
	}
	p.canonicalizeUnalignedUnion(source, delta)
	targetCandidate, err := p.interval.Clone()
	if err != nil {
		return err
	}
	if delta == nil {
		if _, err = targetCandidate.Union(source.interval, widen); err != nil {
			return err
		}
		if err = applyExternalIntervalAssignment(&p.interval, targetCandidate, func() error {
			return p.external.Union(source.external, nil, widen)
		}); err != nil {
			return err
		}
		return nil
	}

	// aligned_union_fn supplies a fresh empty component delta to the basic
	// update, then accumulates that fresh result into the caller's delta using
	// an ordinary union. It never passes the caller delta directly.
	freshInterval := NewIntervalEmpty(uint32(len(p.sorts)))
	if _, err = targetCandidate.UnionWithDelta(source.interval, freshInterval, widen); err != nil {
		return err
	}
	deltaCandidate, err := delta.interval.Clone()
	if err != nil {
		return err
	}
	if _, err = deltaCandidate.Union(freshInterval, false); err != nil {
		return err
	}
	owner := &externalIntervalUnionOwner{runtime: p.plugin.ast.NumeralService().RuntimeOwner()}
	defer func() { resultErr = finishExternalIntervalUnion(owner, resultErr) }()
	owner.fresh, err = p.plugin.NewEmpty(p.sorts)
	if err != nil {
		return err
	}
	if err = applyExternalIntervalAssignment(&p.interval, targetCandidate, func() error {
		return p.external.Union(source.external, owner.fresh, widen)
	}); err != nil {
		return err
	}
	if err = applyExternalIntervalAssignment(&delta.interval, deltaCandidate, func() error {
		return delta.external.Union(owner.fresh, nil, false)
	}); err != nil {
		return err
	}
	return nil
}
