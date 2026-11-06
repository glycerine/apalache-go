package rel

// unionFromConcrete mirrors check_table_plugin::union_fn: each selected
// concrete plugin performs its own union (including LazySparse forcing), then
// both the target and optional delta are checked extensionally.
func (p *FiniteCheckedPair) unionFromConcrete(source, delta *FiniteCheckedPair) error {
	if err := p.verify("union target input"); err != nil {
		return err
	}
	if source == nil {
		return ErrCheckTable
	}
	if err := source.verify("union source input"); err != nil {
		return err
	}
	var primaryDelta, checkerDelta FiniteTable
	if delta != nil {
		if err := delta.verify("union delta input"); err != nil {
			return err
		}
		primaryDelta, checkerDelta = delta.primary, delta.checker
	}
	if err := finiteTableUnion(p.primary, source.primary, primaryDelta); err != nil {
		return err
	}
	if err := finiteTableUnion(p.checker, source.checker, checkerDelta); err != nil {
		return err
	}
	if err := p.verify("union target"); err != nil {
		return err
	}
	if delta != nil {
		return delta.verify("union delta")
	}
	return nil
}
