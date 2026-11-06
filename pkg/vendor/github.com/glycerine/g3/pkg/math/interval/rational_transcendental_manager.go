package interval

// Source: src/math/interval/interval.h:120-170, 336-356.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalTranscendentals is the retained precise config_mpq instantiation
// of the interval manager's transcendental enclosure operations.
type RationalTranscendentals struct {
	Numbers *numeral.RationalManager

	piPrecision uint32
	piHalf      *RationalInterval
	pi          *RationalInterval
	threePiHalf *RationalInterval
	twoPi       *RationalInterval
}

func (r *RationalTranscendentals) validate() error {
	if r == nil || r.Numbers == nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

// Close releases the optional cached pi intervals in source field order.
func (r *RationalTranscendentals) Close() (err error) {
	if r == nil {
		return arena.ErrInvalidSlot
	}
	for _, value := range []*RationalInterval{r.piHalf, r.pi, r.threePiHalf, r.twoPi} {
		if value != nil {
			err = errors.Join(err, value.Close())
		}
	}
	r.piHalf, r.pi, r.threePiHalf, r.twoPi = nil, nil, nil, nil
	r.piPrecision = 0
	return err
}
