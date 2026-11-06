package text

// Concrete counted snapshot of a numeric text stream.

import (
	"bytes"
	"github.com/glycerine/g3/pkg/arena"
)

func CaptureNumericStreamOwnedString(q *OwnedString, s *NumericTextStream, b *arena.Budget) error {
	if q == nil {
		return ErrOwnedStringOwner
	}
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if s.fileMode || s.Buffer == nil {
		return ErrNumericStreamFile
	}
	data := bytes.Clone(s.Buffer.Bytes())
	return q.initialize(b, data)
}
