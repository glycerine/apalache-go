package lp

// Source: src/test/smt_reader.h:71-81,178-292.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// SMTFormulaConstraint owns RHS in Values[0] followed by coefficients in name
// order. Duplicates and zero coefficients remain. Close before the manager.
type SMTFormulaConstraint struct {
	Kind   LConstraintKind
	Names  []string
	Values *IndexedRationalVector
	work   *IndexedRationalVector
	calls  exactEtaCalls
}

func smtInteger(text string) (int32, error) {
	n, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return 0, err
	}
	if n == 0 && (text == "" || text[0] < '0' || text[0] > '9') {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	return int32(n), nil
}

func smtAssert(ok bool) error {
	if !ok {
		return sourceAssertionFailure()
	}
	return nil
}

func (c *SMTFormulaConstraint) addConstant(n int32, add bool) error {
	m := c.Values.manager
	c.work = NewIndexedRationalVector(m, 1)
	if err := m.SetInt32(&c.work.Data[0], n); err != nil {
		return err
	}
	before, err := c.calls.begin(m)
	if err != nil {
		return err
	}
	if add {
		err = m.Add(&c.Values.Data[0], &c.work.Data[0], &c.Values.Data[0])
	} else {
		err = m.Sub(&c.Values.Data[0], &c.work.Data[0], &c.Values.Data[0])
	}
	if err = c.calls.after(m, before, err); err != nil {
		return err
	}
	if err = c.work.Close(); err != nil {
		return err
	}
	c.work = nil
	return nil
}

func (c *SMTFormulaConstraint) addProduct(el SMTLispElement) error {
	if err := smtAssert(len(el.Elements) == 2); err != nil {
		return err
	}
	if len(el.Elements) < 2 {
		return ErrBounds
	}
	coefficient, name := el.Elements[0], el.Elements[1]
	negative := len(coefficient.Elements) != 0
	if negative {
		if err := smtAssert(coefficient.Head == "~" && len(coefficient.Elements) == 1 && len(coefficient.Elements[0].Elements) == 0); err != nil {
			return err
		}
		coefficient = coefficient.Elements[0]
	}
	n, err := smtInteger(coefficient.Head)
	if err != nil {
		return err
	}
	m := c.Values.manager
	c.work = NewIndexedRationalVector(m, 5)
	w := c.work
	if err = m.SetInt32(&w.Data[0], n); err != nil {
		return err
	}
	if negative {
		if err = w.copySlot(&w.Data[1], &w.Data[0]); err != nil {
			return err
		}
		before, e := c.calls.begin(m)
		if e != nil {
			return e
		}
		if err = c.calls.after(m, before, m.Neg(&w.Data[1])); err != nil {
			return err
		}
		w.Data[2] = w.Data[1] // named unary result is moved into local coeff
		w.Data[1] = numeral.NewRational()
	} else {
		w.Data[2] = w.Data[0]
		w.Data[0] = numeral.NewRational()
	}
	for i := 0; i < 2; i++ {
		if err = m.Del(&w.Data[i]); err != nil {
			return err
		}
		w.Data[i] = numeral.NewRational()
	}
	if err = smtAssert(len(name.Elements) == 0); err != nil {
		return err
	}
	parsed, e := strconv.ParseInt(name.Head, 10, 32)
	isInteger := e == nil && parsed != 0 || len(name.Head) > 0 && name.Head[0] >= '0' && name.Head[0] <= '9'
	if isInteger {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	// add_pair takes its coefficient by value, then make_pair copies that
	// lvalue. The pair's move into the vector transfers the final root.
	if err = w.copySlot(&w.Data[3], &w.Data[2]); err != nil {
		return err
	}
	if err = w.copySlot(&w.Data[4], &w.Data[3]); err != nil {
		return err
	}
	if err = c.Values.Resize(uint32(len(c.Names) + 2)); err != nil {
		return err
	}
	c.Values.Data[len(c.Names)+1] = w.Data[4]
	w.Data[4] = numeral.NewRational()
	c.Names = append(c.Names, name.Head)
	if err = w.Close(); err != nil {
		return err
	}
	c.work = nil
	return nil
}

func (c *SMTFormulaConstraint) Close() error {
	m := c.Values.manager
	if err := c.calls.cleanupRaw(m); err != nil {
		return err
	}
	if c.work != nil {
		if err := c.work.Close(); err != nil {
			return err
		}
		c.work = nil
	}
	if err := c.Values.Close(); err != nil {
		return err
	}
	return c.calls.cleanupRoots(m)
}

// Coefficient returns a borrowed slot; it remains valid until the next growth
// or Close. RHS and coefficient ownership stays with this constraint.
func (c *SMTFormulaConstraint) Coefficient(i int) *numeral.Rational {
	if i < 0 || i >= len(c.Names) {
		return nil
	}
	return &c.Values.Data[i+1]
}
