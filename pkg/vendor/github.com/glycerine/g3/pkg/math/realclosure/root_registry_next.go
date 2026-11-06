package realclosure

// Source: src/math/realclosure/realclosure.cpp:724-727.

func (o *algebraicRootOwner) nextAlgebraicIndex() (int, error) {
	if err := o.cleanupTrailingRoots(); err != nil {
		return 0, err
	}
	return len(o.roots), nil
}
