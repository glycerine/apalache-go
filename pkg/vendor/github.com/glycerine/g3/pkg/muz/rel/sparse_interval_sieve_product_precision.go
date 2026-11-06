package rel

// Source: relation_base::is_precise, product_relation::is_precise, and
// sieve_relation's inherited default precision. Raw interval relations are
// approximate; absent full components and sieve-wrapped interval components
// report precise in the source relation hierarchy.

func (p *SparseIntervalSieveProduct) IsPrecise() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.intervalShape != sparseIntervalShapeRaw, nil
}

// AdoptPrecisionShape copies the concrete raw/sieve/absent component shape
// from a same-layout source into an empty target.
func (p *SparseIntervalSieveProduct) AdoptPrecisionShape(source *SparseIntervalSieveProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !p.sameLayout(source) {
		return ErrSparseIntervalProduct
	}
	p.intervalShape = source.intervalShape
	p.intervalNullary = source.intervalNullary
	return nil
}

func (p *SparseIntervalSieveProduct) MarkGeneratedRawShape() error {
	if err := p.open(); err != nil {
		return err
	}
	p.intervalShape = sparseIntervalShapeRaw
	p.intervalNullary = len(p.sorts) == 0
	return nil
}
