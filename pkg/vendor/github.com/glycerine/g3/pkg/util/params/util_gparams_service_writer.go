package params

// Source: src/util/gparams.cpp:446-550,612-631, direct ostream output routes.

import (
	"errors"
	"io"
)

func (s *GlobalParamsService) SourceDisplayTo(out io.Writer, indent uint32, smt2, includeDescr bool) error {
	g, err := s.namespaceImpAt(614)
	if err != nil {
		return err
	}
	if out == nil {
		return ErrGlobalParamsOwner
	}
	w := &sourceTextWriter{dst: out}
	return errors.Join(g.displayInto(w, indent, smt2, includeDescr), w.failure)
}

func (s *GlobalParamsService) SourceDisplayModulesTo(out io.Writer) error {
	g, err := s.namespaceImpAt(619)
	if err != nil {
		return err
	}
	if out == nil {
		return ErrGlobalParamsOwner
	}
	w := &sourceTextWriter{dst: out}
	return errors.Join(g.displayModulesInto(w), w.failure)
}

func (s *GlobalParamsService) SourceDisplayModuleTo(out io.Writer, name string) error {
	g, err := s.namespaceImpAt(624)
	if err != nil {
		return err
	}
	key, err := s.Symbols.Intern(name)
	if err != nil {
		return err
	}
	if out == nil {
		return ErrGlobalParamsOwner
	}
	w := &sourceTextWriter{dst: out}
	g.mu.Lock()
	defer g.mu.Unlock()
	if err = g.open(); err != nil {
		return err
	}
	return errors.Join(g.displayModuleKey(w, key), w.failure)
}

// Both source arguments are std::cout at the shell call site. The same sticky
// writer preserves the unsolicited diagnostic line before the details.
func (s *GlobalParamsService) SourceDisplayParameterTo(out io.Writer, name string) error {
	g, err := s.namespaceImpAt(629)
	if err != nil {
		return err
	}
	if out == nil {
		return ErrGlobalParamsOwner
	}
	w := &sourceTextWriter{dst: out}
	return errors.Join(g.displayParameterInto(w, w, name), w.failure)
}
