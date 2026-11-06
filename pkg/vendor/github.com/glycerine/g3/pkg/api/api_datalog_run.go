package api

import (
	"bytes"

	"github.com/glycerine/g3/pkg/shell"
)

// RunDatalogPath parses either one .dl/.datalog file or a WPA directory,
// saturates once, and returns copied finite relation tables.
func (c *SMTContext) RunDatalogPath(path string) (result DatalogRunResult, err error) {
	if err = c.open(); err != nil {
		return result, err
	}
	var output bytes.Buffer
	frontend, err := shell.NewDatalogFrontend(c.command, &output)
	if err != nil {
		return result, err
	}
	if err = frontend.Run(path); err != nil {
		result.Output = output.String()
		return result, err
	}
	predicates, err := frontend.Results()
	if err != nil {
		return result, err
	}
	snapshots, err := frontend.RelationSnapshots()
	if err != nil {
		return result, err
	}
	result.Relations = make([]DatalogRelationResult, len(predicates))
	for i := range predicates {
		result.Relations[i] = DatalogRelationResult{Name: predicates[i].Name, Status: predicates[i].Status, Facts: snapshots[i].Facts}
	}
	result.Timings, err = frontend.Timings()
	result.Output = output.String()
	return result, err
}
