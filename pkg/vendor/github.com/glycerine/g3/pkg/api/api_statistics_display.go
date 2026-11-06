package api

// Source: src/api/api_stats.cpp:26-38.

import (
	"strings"
)

func (s *APIStatistics) String() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	if err := s.value.DisplaySMT2(&output); err != nil {
		return "", err
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
}
