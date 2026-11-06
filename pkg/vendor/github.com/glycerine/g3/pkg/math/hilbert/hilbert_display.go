package hilbert

import (
	"fmt"
	"io"
)

// Display emits the source diagnostic sections that remain meaningful with
// Go-owned vector storage.
func (b *Basis) Display(out io.Writer) error {
	if out == nil {
		return nil
	}
	if _, err := io.WriteString(out, "inequalities:\n"); err != nil {
		return err
	}
	for _, row := range b.inequalities {
		if err := displayInequality(out, row); err != nil {
			return err
		}
	}
	if len(b.basis) != 0 {
		if _, err := io.WriteString(out, "basis:\n"); err != nil {
			return err
		}
		for _, value := range b.basis {
			for _, coordinate := range value.values {
				if _, err := fmt.Fprintf(out, "%d ", coordinate); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(out, "-> %d\n", value.weight); err != nil {
				return err
			}
		}
	}
	return nil
}

func displayInequality(out io.Writer, row inequality) error {
	for j := 1; j < len(row.values); j++ {
		value := row.values[j]
		if value == 0 {
			continue
		}
		if value > 0 {
			if _, err := io.WriteString(out, " + "); err != nil {
				return err
			}
		} else {
			if _, err := io.WriteString(out, " - "); err != nil {
				return err
			}
			value = -value
		}
		if value != 1 {
			if _, err := fmt.Fprintf(out, "%d*", value); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "x%d", j); err != nil {
			return err
		}
	}
	relation := ">="
	if row.equal {
		relation = "="
	}
	_, err := fmt.Fprintf(out, " %s %d\n", relation, -row.values[0])
	return err
}
