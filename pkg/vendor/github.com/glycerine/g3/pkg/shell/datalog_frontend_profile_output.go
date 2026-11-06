package shell

import (
	"fmt"
	"io"
)

// WriteProfile renders the retained verbose relation-size section explicitly.
func (f *DatalogFrontend) WriteProfile(out io.Writer) error {
	if out == nil {
		return ErrDatalogFrontend
	}
	profile, err := f.Profile()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "--------------\nrelation sizes\n"); err != nil {
		return err
	}
	for _, relation := range profile.Relations {
		if _, err = fmt.Fprintf(out, "%s: %d\n", relation.Name, relation.TupleCount); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(out, "rules: %d\nrelations: %d\ntuples: %d\ncompiled: %t\n", profile.RuleCount, profile.RelationCount, profile.TupleCount, profile.Compiled)
	return err
}
