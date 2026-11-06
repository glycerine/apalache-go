package api

// Source: src/api/api_solver.cpp:452-464; src/api/api_datalog.cpp:419-428.

import "github.com/glycerine/g3/pkg/util/statistics"

// StatisticsObject snapshots solver, process-memory, and resource-limit
// counters into the source API statistics object shape.
func (c *SMTContext) StatisticsObject() (*APIStatistics, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.Statistics()
	if err != nil {
		return nil, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return nil, err
	}
	if numerals := manager.NumeralService(); numerals != nil && numerals.RuntimeOwner() != nil {
		if process := numerals.RuntimeOwner().ProcessOwner(); process != nil {
			if err = statistics.AddMemory(&value, process); err != nil {
				return nil, err
			}
		}
	}
	statistics.AddResourceLimit(&value, manager.ResourceLimit().Count())
	return newAPIStatistics(c.smtContextState, value)
}

func (c *SMTContext) FixedpointStatisticsObject() (*APIStatistics, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.FixedpointStatistics()
	if err != nil {
		return nil, err
	}
	return newAPIStatistics(c.smtContextState, value)
}
