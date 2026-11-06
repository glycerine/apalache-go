package fp

// Source: local rule-set query copies preserve registered predicates.

func (e *PropositionalEngine) copyQueryPredicates(clone *PropositionalEngine) error {
	for _, declaration := range e.predicateOrder {
		if err := clone.RegisterPredicate(declaration); err != nil {
			return err
		}
		output, err := e.set.IsOutputPredicate(declaration)
		if err != nil {
			return err
		}
		if output {
			if err = clone.SetOutputPredicate(declaration); err != nil {
				return err
			}
		}
		clone.predicates[declaration].argumentNames = append(clone.predicates[declaration].argumentNames[:0], e.predicates[declaration].argumentNames...)
		clone.predicates[declaration].argumentNamesSet = e.predicates[declaration].argumentNamesSet
		clone.predicates[declaration].requestedRepresentations = append([]string(nil), e.predicates[declaration].requestedRepresentations...)
		clone.predicates[declaration].explicitFiniteRepresentation = e.predicates[declaration].explicitFiniteRepresentation
		if e.backend == "datalog" && e.predicates[declaration].external != nil {
			if clone.external == nil {
				return ErrPropositionalHorn
			}
			relation, err := clone.external.PredicateRelation(declaration)
			if err != nil {
				return err
			}
			clone.predicates[declaration].external = relation
			clone.predicates[declaration].table = nil
		}
		if e.backend == "datalog" && e.predicates[declaration].table != nil {
			table, err := clone.cloneFiniteForContext(e.predicates[declaration].table)
			if err != nil {
				return err
			}
			if err = clone.installDatalogTable(declaration, table); err != nil {
				return err
			}
		}
		if e.auxiliary[declaration] {
			clone.auxiliary[declaration] = true
		}
	}
	return nil
}
