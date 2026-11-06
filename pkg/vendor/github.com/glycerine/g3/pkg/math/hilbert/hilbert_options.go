package hilbert

// Source: src/math/hilbert/hilbert_basis.h public configuration surface.

func (b *Basis) SetUseSupport(enabled bool) { b.useSupport = enabled }

func (b *Basis) SetUseOrderedSupport(enabled bool) { b.orderedSupport = enabled }

// SetUseOrderedSubsumption preserves the source option. The source field is
// diagnostic-only in this revision; both settings therefore use the same
// dominance relation.
func (b *Basis) SetUseOrderedSubsumption(enabled bool) { b.orderedSubsumption = enabled }
