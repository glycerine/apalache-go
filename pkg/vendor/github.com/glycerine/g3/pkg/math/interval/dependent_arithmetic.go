package interval

// Source: src/math/interval/dep_intervals.h:217-290, 393-406.

import "github.com/glycerine/g3/pkg/util/numeral"

func (v *DependentRationalInterval) Add(a, b *DependentRationalInterval, withDeps bool) error {
	if err := v.checkOperands(a, b); err != nil {
		return err
	}
	deps, err := a.Value.AddDeps(b.Value)
	if err != nil {
		return err
	}
	ad0, ad1, bd0, bd1 := a.LowerDep, a.UpperDep, b.LowerDep, b.UpperDep
	if err = v.Value.Add(a.Value, b.Value); err != nil {
		return err
	}
	if withDeps {
		aa, bb := *a, *b
		aa.LowerDep, aa.UpperDep, bb.LowerDep, bb.UpperDep = ad0, ad1, bd0, bd1
		v.manager.publishBinaryDeps(&aa, &bb, v, deps)
	}
	return nil
}

func (v *DependentRationalInterval) Mul(a, b *DependentRationalInterval, withDeps bool) error {
	if err := v.checkOperands(a, b); err != nil {
		return err
	}
	deps, err := a.Value.MulDeps(b.Value)
	if err != nil {
		return err
	}
	ad0, ad1, bd0, bd1 := a.LowerDep, a.UpperDep, b.LowerDep, b.UpperDep
	if err = v.Value.Mul(a.Value, b.Value); err != nil {
		return err
	}
	if withDeps {
		aa, bb := *a, *b
		aa.LowerDep, aa.UpperDep, bb.LowerDep, bb.UpperDep = ad0, ad1, bd0, bd1
		v.manager.publishBinaryDeps(&aa, &bb, v, deps)
	}
	return nil
}

func (v *DependentRationalInterval) Div(a, b *DependentRationalInterval, withDeps bool) error {
	if err := v.checkOperands(a, b); err != nil {
		return err
	}
	deps, err := a.Value.DivDeps(b.Value)
	if err != nil {
		return err
	}
	ad0, ad1, bd0, bd1 := a.LowerDep, a.UpperDep, b.LowerDep, b.UpperDep
	if err = v.Value.Div(a.Value, b.Value); err != nil {
		return err
	}
	if withDeps {
		aa, bb := *a, *b
		aa.LowerDep, aa.UpperDep, bb.LowerDep, bb.UpperDep = ad0, ad1, bd0, bd1
		v.manager.publishBinaryDeps(&aa, &bb, v, deps)
	}
	return nil
}

func (v *DependentRationalInterval) Power(a *DependentRationalInterval, n uint32, withDeps bool) error {
	if err := v.checkPeer(a); err != nil {
		return err
	}
	deps, err := a.Value.PowerDeps(n)
	if err != nil {
		return err
	}
	lower, upper := a.LowerDep, a.UpperDep
	if err = v.Value.Power(a.Value, n); err != nil {
		return err
	}
	if withDeps {
		snapshot := *a
		snapshot.LowerDep, snapshot.UpperDep = lower, upper
		v.manager.publishBinaryDeps(&snapshot, &snapshot, v, deps)
	}
	return nil
}

func (v *DependentRationalInterval) MulScalar(k *numeral.Rational, a *DependentRationalInterval, withDeps bool) error {
	if err := v.checkPeer(a); err != nil {
		return err
	}
	deps, err := a.Value.MulScalarDeps(k)
	if err != nil {
		return err
	}
	snapshot := *a
	if err := v.Value.MulScalar(k, a.Value); err != nil {
		return err
	}
	if withDeps {
		v.manager.publishBinaryDeps(&snapshot, &snapshot, v, deps)
	}
	return nil
}

func (v *DependentRationalInterval) checkOperands(a, b *DependentRationalInterval) error {
	if err := v.checkPeer(a); err != nil {
		return err
	}
	return v.checkPeer(b)
}
