package rel

// Source: finite_product_relation_plugin::to_table_relation (211–229).
// The registered table plugin projects the functional inner-relation index;
// check_table independently performs that projection on both concrete sides.

// ToFiniteTable removes the functional index after all explanation columns
// have been projected away. The returned table owns its storage independently
// and preserves the concrete projection plugin, including a checked pair.
func (p *FiniteExplanationProduct) ToFiniteTable() (FiniteTable, error) {
	can, err := p.CanConvertToTableRelation()
	if err != nil {
		return nil, err
	}
	if !can {
		return nil, ErrTableRelation
	}
	if err = p.collectEmptyExplanationRows(); err != nil {
		return nil, err
	}
	switch table := p.table.(type) {
	case *CheckedExplanationIndex:
		return table.ProjectFinite()
	case *SparseTable:
		return finiteTableProject(table, []int{len(p.finiteSorts)})
	case *LazySparseTable:
		return finiteTableProject(table, []int{len(p.finiteSorts)})
	default:
		return nil, ErrTableRelation
	}
}
