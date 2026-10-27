package apalache

// CommonSupertype implements Apalache's flexible equality: ordinary types
// must unify, while record and sparse-tuple shapes may widen to their union.
func CommonSupertype(left, right Type) (Type, bool, error) {
	_, unified, ok, err := NewTypeUnifier(NewTypeVarPool(0)).Unify(EmptySubstitution(), left, right)
	return unified, ok, err
}

func CompatibleTypes(left, right Type) bool {
	_, ok, err := CommonSupertype(left, right)
	return err == nil && ok
}

func CommonSequenceSupertype(types []Type) (Type, bool, error) {
	if len(types) == 0 {
		return nil, false, nil
	}
	result := types[0]
	for _, typ := range types[1:] {
		var ok bool
		var err error
		result, ok, err = CommonSupertype(result, typ)
		if err != nil || !ok {
			return nil, ok, err
		}
	}
	return result, true, nil
}
