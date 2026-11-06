package api

// Source: src/api/api_stats.cpp:41-62; src/api/api_stats.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// APIStatistics is an owning snapshot corresponding to Z3_stats_ref.
type APIStatistics struct {
	owner  *smtContextState
	value  statistics.Statistics
	closed bool
}

func newAPIStatistics(owner *smtContextState, source statistics.Statistics) (*APIStatistics, error) {
	if owner == nil || owner.closed || owner.command == nil {
		return nil, ErrSMTContextOwner
	}
	result := &APIStatistics{owner: owner}
	result.value.Copy(source)
	return result, nil
}

func (s *APIStatistics) open() error {
	if s == nil || s.owner == nil || s.closed || s.owner.closed || s.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (s *APIStatistics) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.value.Reset()
	s.owner = nil
	s.closed = true
	return nil
}

func (s *APIStatistics) Size() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	n := s.value.Size()
	if uint64(n) > uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(n), nil
}
