package numeral

// Source: src/util/mpq.cpp:130-141; explicit Go owning continuation.

// CleanupValue owns only this caller's actual NUM/DEN fields. Unjoined lower
// allocations remain with their real manager scopes; none is adopted here.
func (s *RationalService) CleanupValue(v *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := s.Manager.Z.CleanupRawNumeral(&v.Raw.Num); e != nil {
		return e
	}
	if e := s.Manager.Z.CleanupRawNumeral(&v.Raw.Den); e != nil {
		return e
	}
	*v = OwnedRational{}
	return nil
}
