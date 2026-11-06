package realclosure

// Source: value_ref ownership surrounding realclosure.cpp:3440-3463.

import "errors"

func (s *realValueStore) releaseAlgebraicIntervalQuery(result *algebraicIntervalQuery) error {
	if result == nil {
		return nil
	}
	err := errors.Join(s.releaseSturmScratch(result.Lower), s.releaseSturmScratch(result.Upper))
	if err == nil {
		*result = algebraicIntervalQuery{}
	}
	return err
}
