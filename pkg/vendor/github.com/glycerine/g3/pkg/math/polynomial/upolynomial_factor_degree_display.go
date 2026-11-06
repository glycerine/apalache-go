package polynomial

// Source: upolynomial_factorization_int.h:111-119; bit_vector.cpp:188-201.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

// Display borrows the writer for this call. The logical bits remain native Go
// flags; only their source textual order is retained.
func (d *UPolynomialDegreeSet) Display(out io.Writer) error {
	if e := d.checkMutable(); e != nil {
		return e
	}
	d.displaying = true
	defer func() { d.displaying = false }()
	return d.state.display(out)
}

func (s *upolyFactorDegreeState) display(out io.Writer) error {
	if e := linearWriteText(out, "[0"); e != nil {
		return e
	}
	// Source max_degree underflows on an unconstructed empty set. Retained
	// callers have at least bit zero; keep a separate native extent guard.
	if len(s.values) == 0 {
		return arena.ErrBounds
	}
	for i := uint32(1); i <= uint32(len(s.values))-1; i++ {
		included, e := s.get(i)
		if e != nil {
			return e
		}
		if included {
			if e = linearWriteText(out, ", "); e != nil {
				return e
			}
			if e = linearWriteText(out, strconv.FormatUint(uint64(i), 10)); e != nil {
				return e
			}
		}
	}
	if e := linearWriteText(out, "] represented by "); e != nil {
		return e
	}
	for i := uint32(len(s.values)); i > 0; {
		i--
		included, e := s.get(i)
		if e != nil {
			return e
		}
		bit := "0"
		if included {
			bit = "1"
		}
		if e = linearWriteText(out, bit); e != nil {
			return e
		}
	}
	return nil
}
