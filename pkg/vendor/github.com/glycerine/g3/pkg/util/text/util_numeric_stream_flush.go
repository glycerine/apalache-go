package text

// Source: src/util/debug.cpp:79-80 (formatted std::cin sentry tie()->flush()).

// Flush is the selected direct-emission stream adapter. Every prior file byte
// has already reached os.File.Write; the memory stream similarly has no pending
// provider writes. This unformatted action validates the actual output owner
// without consuming width, changing flags/badbit or writing, closing, syncing
// or fsyncing the borrowed file. Native buffering/pub-sync/ties remain explicit
// representation obligations rather than a synthetic flush counter or callback.
func (s *NumericTextStream) Flush() error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return nil
}
