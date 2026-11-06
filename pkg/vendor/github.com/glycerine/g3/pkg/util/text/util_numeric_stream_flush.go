package text

// Source: src/util/debug.cpp:79-80 (formatted std::cin sentry tie()->flush()).

// Flush publishes a borrowed writer if it supplies Flush. File bytes are
// written directly, so the borrowed file is neither closed nor fsynced.
func (s *NumericTextStream) Flush() error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if f, ok := s.writer.(interface{ Flush() error }); ok && !s.FileBad && !s.fileFail {
		if f.Flush() != nil {
			s.FileBad = true
		}
	}
	return nil
}
