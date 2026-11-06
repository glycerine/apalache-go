package rel

// SetRelationExplanationUDocFactory installs the context-owned concrete DOC
// plugin used by relation-level explanation sieves. Existing products borrow
// the factory, so replacing an active owner is rejected.
func (m *RelationManager) SetRelationExplanationUDocFactory(factory *UDocRelationFactory) error {
	if err := m.open(); err != nil || factory == nil || factory.open() != nil || factory.manager != m.manager {
		return ErrUDocTableProduct
	}
	if m.explanationUDoc != nil && m.explanationUDoc != factory && len(m.relationLevel) != 0 {
		return ErrUDocTableProduct
	}
	m.explanationUDoc = factory
	m.explanationUDocOwned = true
	return nil
}

// BorrowRelationExplanationUDocFactory installs a source-owned factory for
// query-cloned managers. The clone may create products with it but must not
// close the factory shared with the source context.
func (m *RelationManager) BorrowRelationExplanationUDocFactory(factory *UDocRelationFactory) error {
	if err := m.open(); err != nil || factory == nil || factory.open() != nil || factory.manager != m.manager {
		return ErrUDocTableProduct
	}
	if m.explanationUDoc != nil && m.explanationUDoc != factory && len(m.relationLevel) != 0 {
		return ErrUDocTableProduct
	}
	m.explanationUDoc = factory
	m.explanationUDocOwned = false
	return nil
}

func (m *RelationManager) RelationExplanationUDocFactory() (*UDocRelationFactory, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	if m.explanationUDoc == nil {
		return nil, false, nil
	}
	if err := m.explanationUDoc.open(); err != nil {
		return nil, false, err
	}
	return m.explanationUDoc, true, nil
}
