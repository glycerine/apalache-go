package opt

// Source: src/opt/pb_sls.cpp operator() and collect_statistics.

import "github.com/glycerine/g3/pkg/util/lbool"

func (s *maxResSLSCandidateSearch) check() (lbool.LBool, error) {
	status, err := s.search.Check()
	statistics, statsErr := s.search.Statistics()
	if statsErr == nil {
		s.engine.stats.SLSFlips += uint32(statistics["sls.num_flips"])
		s.engine.stats.SLSImprovements += uint32(statistics["sls.num_improvements"])
	}
	return status, joinMaxResSLSErrors(err, statsErr)
}
