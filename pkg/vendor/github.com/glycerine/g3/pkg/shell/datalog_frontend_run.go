package shell

import (
	"time"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *DatalogFrontend) Saturate() error {
	if f == nil || !f.loaded || f.saturated {
		return ErrDatalogFrontend
	}
	printTuples, err := f.context.FixedpointDatalogPrintTuples()
	if err != nil {
		return err
	}
	predicates, err := f.context.FixedpointPredicates()
	if err != nil {
		return err
	}
	if _, err = f.context.QueryFixedpointRelations(predicates); err != nil {
		return err
	}
	f.snapshots, err = f.context.FixedpointRelationSnapshots(predicates)
	if err != nil {
		return err
	}
	f.results = make([]DatalogPredicateResult, 0, len(predicates))
	manager, err := f.context.Manager()
	if err != nil {
		return err
	}
	for i, declaration := range predicates {
		status := lbool.FromBool(len(f.snapshots[i].Facts) != 0)
		name, queryErr := manager.GetASTName(declaration)
		if queryErr != nil {
			return queryErr
		}
		text, queryErr := manager.Symbols.String(name)
		if queryErr != nil {
			return queryErr
		}
		f.results = append(f.results, DatalogPredicateResult{Declaration: declaration, Name: text, Status: status})
	}
	f.runFinished, f.saturated, f.printTuples = time.Now(), true, printTuples
	return nil
}

func (f *DatalogFrontend) Run(path string) error {
	if err := f.Load(path); err != nil {
		return err
	}
	if err := f.Saturate(); err != nil {
		return err
	}
	if err := f.WriteTuples(f.output); err != nil {
		return err
	}
	return f.WriteStatistics()
}
