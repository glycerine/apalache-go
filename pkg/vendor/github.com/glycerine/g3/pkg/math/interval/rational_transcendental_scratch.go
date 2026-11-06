package interval

// Source: src/math/interval/interval.h:120-170 scoped numeral temporaries.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type rationalTranscendentalScratch struct {
	numbers *numeral.RationalManager
	values  []*numeral.Rational
}

func (s *rationalTranscendentalScratch) new() *numeral.Rational {
	v := numeral.NewRational()
	s.values = append(s.values, &v)
	return &v
}

func (s *rationalTranscendentalScratch) close() (err error) {
	for i := len(s.values) - 1; i >= 0; i-- {
		err = errors.Join(err, s.numbers.Del(s.values[i]))
	}
	s.values = nil
	return err
}
