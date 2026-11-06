package api

// Source: src/api/api_stats.cpp:65-98.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (s *APIStatistics) index(index uint32) (int, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if uint64(index) >= uint64(s.value.Size()) {
		return 0, arena.ErrBounds
	}
	return int(index), nil
}

func (s *APIStatistics) Key(index uint32) (string, error) {
	i, err := s.index(index)
	if err != nil {
		return "", err
	}
	return string(s.value.Key(i)), nil
}

func (s *APIStatistics) IsUint(index uint32) (bool, error) {
	i, err := s.index(index)
	if err != nil {
		return false, err
	}
	return s.value.IsUint(i), nil
}

func (s *APIStatistics) IsDouble(index uint32) (bool, error) {
	isUint, err := s.IsUint(index)
	return !isUint && err == nil, err
}
