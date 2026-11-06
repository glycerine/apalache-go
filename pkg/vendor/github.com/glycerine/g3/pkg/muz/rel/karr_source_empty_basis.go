package rel

// PrepareSourceEmptyBasis handles the native Karr dualizeI boundary for a
// relation with no inequality rows. The native Hilbert basis has zero
// variables there, so saturation produces no generators. It is used by the
// Datalog unbound-column consumer at the source get_basis call sites; other
// consumers retain their existing full-relation treatment.
func (r *KarrRelation) PrepareSourceEmptyBasis() {
	if r == nil || r.empty || r.basisValid || !r.inequalitiesValid || r.inequalities.Len() != 0 {
		return
	}
	// Native saturate checks the resource limit at loop entry and again on exit.
	if r.checkpoint != nil {
		r.checkpoint()
		r.checkpoint()
	}
	r.basis = KarrMatrix{}
	r.basisValid = true
}

// KarrSourceProject and KarrSourceUnion implement the native get_basis
// boundaries used by the concrete Datalog compiler. Other consumers continue
// to use the general typed operations.
func KarrSourceProject(source *KarrRelation, removed []uint32) (*KarrRelation, error) {
	if source == nil {
		return nil, ErrKarrDimension
	}
	// Validate before preparing the borrowed source, just as Project does.
	if err := validateKarrColumns(source.dimension, removed); err != nil {
		return nil, err
	}
	seen := make(map[uint32]bool, len(removed))
	for _, column := range removed {
		if seen[column] {
			return nil, ErrKarrColumn
		}
		seen[column] = true
	}
	source.PrepareSourceEmptyBasis()
	return source.Project(removed)
}

func KarrSourceUnion(target, source *KarrRelation) (bool, error) {
	if target == nil || source == nil || target.dimension != source.dimension {
		return false, ErrKarrDimension
	}
	if source.empty {
		return false, nil
	}
	source.PrepareSourceEmptyBasis()
	// Native union gets the source basis before touching the destination and
	// does not recheck empty if basis construction discovers inconsistency.
	sourceBasis, err := source.Basis()
	if err != nil {
		return false, err
	}
	if target.empty {
		target.empty = false
		target.basis = sourceBasis
		target.basisValid = true
		target.inequalitiesValid = false
		return true, nil
	}
	target.PrepareSourceEmptyBasis()
	if err := target.ensureBasis(); err != nil {
		return false, err
	}
	oldSize := target.basis.Len()
	for i := range sourceBasis.A {
		found := false
		for j := 0; !found && j < oldSize; j++ {
			found = sameKarrRow(sourceBasis.A[i], &sourceBasis.B[i], sourceBasis.Eq[i], target.basis.A[j], &target.basis.B[j], target.basis.Eq[j])
		}
		if !found {
			target.basis.append(sourceBasis.A[i], &sourceBasis.B[i], sourceBasis.Eq[i])
		}
	}
	// The native union invalidates inequalities even when no row is added.
	target.inequalitiesValid = false
	return oldSize != target.basis.Len(), nil
}
