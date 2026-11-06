package nlsat

// Source: src/nlsat/nlsat_solver.cpp:504-508.

func (f *atomFactory) deleteUnreferenced() error {
	if err := f.open(); err != nil {
		return err
	}
	for _, atom := range append([]ArithmeticAtom(nil), f.bools.atoms...) {
		if atom == nil {
			continue
		}
		if arithmeticAtomHeader(atom).refs != 0 {
			return ErrAtomCacheLive
		}
		if err := f.deleteAtom(atom); err != nil {
			return err
		}
	}
	return nil
}
