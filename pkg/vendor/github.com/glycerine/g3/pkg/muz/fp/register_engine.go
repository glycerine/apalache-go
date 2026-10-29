package fp

// Source: src/muz/fp/dl_register_engine.cpp concrete fixedpoint engine factory.

import "fmt"

// SelectEngine applies the retained fixedpoint engine factory branch.
func (e *PropositionalEngine) SelectEngine(name string) error {
	if err := e.open(); err != nil {
		return err
	}
	switch name {
	case "auto-config":
		e.backend = "datalog"
		for _, state := range e.predicates {
			if state.table == nil {
				if e.params == nil || e.globals == nil {
					return fmt.Errorf("muz/fp: fixedpoint auto-config requires parameter services for infinite-domain predicates")
				}
				e.backend = "pdr"
				break
			}
		}
		return nil
	case "datalog":
		e.backend = "datalog"
		return nil
	case "pdr", "spacer":
		if e.params == nil || e.globals == nil {
			return fmt.Errorf("muz/fp: fixedpoint engine %s requires parameter services", name)
		}
		e.backend = name
		return nil
	case "bmc", "tab", "clp", "duality", "ddnf", "qpdr", "qbmc":
		return fmt.Errorf("muz/fp: fixedpoint engine %s is not ported", name)
	default:
		return fmt.Errorf("muz/fp: unknown fixedpoint engine %s", name)
	}
}
