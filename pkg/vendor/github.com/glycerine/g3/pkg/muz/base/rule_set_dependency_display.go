package base

import (
	"fmt"
	"io"
)

func (s *RuleSet) WriteDependencies(out io.Writer) error {
	if out == nil {
		return fmt.Errorf("muz/base: nil dependency display writer")
	}
	strata, err := s.Strata()
	if err != nil {
		return err
	}
	for _, stratum := range strata {
		nonempty := false
		for _, first := range stratum {
			firstName, e := s.manager.manager.GetASTName(first)
			if e != nil {
				return e
			}
			firstText, e := s.manager.manager.Symbols.Display(firstName)
			if e != nil {
				return e
			}
			for _, dependency := range s.dependencies[first] {
				dependencyName, x := s.manager.manager.GetASTName(dependency)
				if x != nil {
					return x
				}
				dependencyText, x := s.manager.manager.Symbols.Display(dependencyName)
				if x != nil {
					return x
				}
				if _, x = io.WriteString(out, firstText+" -> "+dependencyText+"\n"); x != nil {
					return x
				}
				nonempty = true
			}
		}
		if nonempty {
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
