package polynomial

// Source: src/math/polynomial/polynomial.h:62-69; src/math/polynomial/polynomial.cpp:66-82; src/util/vector.h:101-116,160-210,530-546.

// PolynomialVariableDegreesTextSink is a borrowed output binding.
type PolynomialVariableDegreesTextSink interface {
	AppendCString(string) error
	AppendUint32(uint32) error
}

func (d *PolynomialVariableDegrees) DisplayInto(out PolynomialVariableDegreesTextSink) error {
	if e := d.open(); e != nil {
		return e
	}
	if out == nil {
		return ErrPolynomialOwner
	}
	if e := out.AppendCString("["); e != nil {
		return e
	}
	for i := 0; i < len(d.values); i++ {
		if i != 0 {
			if e := out.AppendCString(","); e != nil {
				return e
			}
		}
		if e := out.AppendCString("x"); e != nil {
			return e
		}
		if e := out.AppendUint32(uint32(i)); e != nil {
			return e
		}
		if e := out.AppendCString("^"); e != nil {
			return e
		}
		if i >= len(d.values) {
			return ErrPolynomialAssertion
		}
		if e := out.AppendUint32(d.values[i]); e != nil {
			return e
		}
	}
	return out.AppendCString("]")
}

func NewPolynomialVariableDegreesCopy(source *PolynomialVariableDegrees) (*PolynomialVariableDegrees, error) {
	if e := source.open(); e != nil {
		return nil, e
	}
	d, e := NewPolynomialVariableDegrees(source.runtime)
	if e != nil {
		return nil, e
	}
	d.values = append(d.values, source.values...)
	return d, nil
}
func (d *PolynomialVariableDegrees) CopyFrom(source *PolynomialVariableDegrees) error {
	if e := d.open(); e != nil {
		return e
	}
	if d == source {
		return nil
	}
	if e := source.open(); e != nil {
		return e
	}
	d.values = append(d.values[:0], source.values...)
	return nil
}
func NewPolynomialVariableDegreesMove(source *PolynomialVariableDegrees) (*PolynomialVariableDegrees, error) {
	if e := source.open(); e != nil {
		return nil, e
	}
	d, e := NewPolynomialVariableDegrees(source.runtime)
	if e != nil {
		return nil, e
	}
	d.values, source.values = source.values, nil
	return d, nil
}

// Source move assignment resolves to svector<unsigned>::operator=(const&), so
// it remains a deep copy. Only move construction transfers slice ownership.
func (d *PolynomialVariableDegrees) MoveFrom(source *PolynomialVariableDegrees) error {
	return d.CopyFrom(source)
}
