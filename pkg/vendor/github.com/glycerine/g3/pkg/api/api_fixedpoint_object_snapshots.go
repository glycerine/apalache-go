package api

// Source: src/api/api_datalog.cpp fixedpoint rule/assertion/statistics access.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (f *APIFixedpoint) Assertions() ([]ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return f.command.FixedpointAssertions()
}

func (f *APIFixedpoint) Rules() ([]ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return f.command.FixedpointRules()
}

func (f *APIFixedpoint) Statistics() (statistics.Statistics, error) {
	if err := f.open(); err != nil {
		return statistics.Statistics{}, err
	}
	return f.command.FixedpointStatistics()
}

func (f *APIFixedpoint) StatisticsObject() (*APIStatistics, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	value, err := f.command.FixedpointStatistics()
	if err != nil {
		return nil, err
	}
	return newAPIStatistics(f.owner, value)
}
