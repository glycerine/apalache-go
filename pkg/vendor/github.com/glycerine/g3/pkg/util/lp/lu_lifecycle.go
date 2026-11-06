package lp

// Source: src/util/lp/lu.hpp:936-944; explicit instances in lu_instances.cpp:39-41.

// InitFloatFactorization destroys the old factorization before constructing its
// replacement. On error, the slot retains the partial new owner for Close.
func InitFloatFactorization(slot **FloatLU, a *FloatStaticMatrix, basis []uint32, s *Settings) error {
	if *slot != nil {
		// Floating Close releases native Go storage and always succeeds.
		(*slot).Close()
	}
	var err error
	*slot, err = NewFloatLU(a, basis, s)
	return err
}

func InitRationalFactorization(slot **RationalLU, a *RationalStaticMatrix, basis []uint32, s *Settings) error {
	if *slot != nil {
		if err := (*slot).Close(); err != nil {
			return err
		}
	}
	var err error
	*slot, err = NewRationalLU(a, basis, s)
	return err
}

func InitPairFactorization(slot **PairLU, a *RationalStaticMatrix, basis []uint32, s *Settings) error {
	if *slot != nil {
		if err := (*slot).Close(); err != nil {
			return err
		}
	}
	var err error
	*slot, err = NewPairLU(a, basis, s)
	return err
}
