package realclosure

// Source: src/math/realclosure/realclosure.cpp:3207-3230.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) lcmIntegerDenominators(values []*realValue) (result *realValue, err error) {
	lcm := numeral.NewRational()
	defer func() {
		if lcm.Num.Val != 0 {
			err = errors.Join(err, s.releaseSturmRational(&lcm))
		}
	}()
	if err = s.numbers.SetInt32(&lcm, 1); err != nil {
		return nil, err
	}
	for _, value := range values {
		if value == nil || !value.isRational || value.store != s || value.deleted || value.refs == 0 {
			return nil, arena.ErrInvalidSlot
		}
		integer, e := s.numbers.IsInt(&value.rational)
		if e != nil || !integer {
			return nil, errors.Join(e, arena.ErrInvalidSlot)
		}
		next := numeral.NewRational()
		if e = s.numbers.LCM(&lcm, &value.rational, &next); e != nil {
			return nil, errors.Join(e, s.releaseSturmRational(&next))
		}
		if e = s.releaseSturmRational(&lcm); e != nil {
			lcm = numeral.NewRational()
			return nil, errors.Join(e, s.releaseSturmRational(&next))
		}
		lcm = next
	}
	result, err = s.ownNonzeroRational(&lcm)
	if err == nil {
		lcm = numeral.NewRational()
	}
	return result, err
}
