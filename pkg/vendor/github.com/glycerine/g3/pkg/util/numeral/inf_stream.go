package numeral

// Source: src/util/inf_rational.h:459-463; src/util/inf_int_rational.h:374-378;
// src/util/inf_eps_rational.h:431-435.

import utext "github.com/glycerine/g3/pkg/util/text"

// The source stream operators first construct to_string(), then insert that
// complete string once. In particular, width applies to the entire numeral.
func (s *RationalService) InfDisplay(out *utext.NumericTextStream, v *OwnedInfRational) error {
	value, err := s.InfString(v)
	if err != nil {
		return err
	}
	if out == nil {
		return utext.ErrNumericStreamOwner
	}
	return out.AppendString(value)
}

func (s *RationalService) InfIntDisplay(out *utext.NumericTextStream, v *OwnedInfIntRational) error {
	value, err := s.InfIntString(v)
	if err != nil {
		return err
	}
	if out == nil {
		return utext.ErrNumericStreamOwner
	}
	return out.AppendString(value)
}

func (s *RationalService) InfEpsInfDisplay(out *utext.NumericTextStream, v *OwnedInfEpsInfRational) error {
	value, err := s.InfEpsInfString(v)
	if err != nil {
		return err
	}
	if out == nil {
		return utext.ErrNumericStreamOwner
	}
	return out.AppendString(value)
}
