package rel

import "math/big"

// KarrSourceFilterConstraint appends a row as karr_relation::filter_interpreted
// does. The source retains any valid cached basis when it appends this row.
// The Datalog instruction does not invoke the filter for an empty register.
func KarrSourceFilterConstraint(r *KarrRelation, row []big.Rat, constant *big.Rat, equal bool) error {
	if r == nil || uint32(len(row)) != r.dimension {
		return ErrKarrDimension
	}
	if r.empty {
		return nil
	}
	if err := r.ensureInequalities(); err != nil {
		return err
	}
	r.inequalities.append(row, constant, equal)
	return nil
}
