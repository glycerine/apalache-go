package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

type intervalRestoreEntry struct {
	target *BinaryInterval
	saved  *BinaryInterval
}

type intervalRestoreFrame struct {
	previous   *intervalRestoreFrame
	values     []intervalRestoreEntry
	extensions []intervalRestoreEntry
	valueSet   map[*BinaryInterval]bool
	extSet     map[*BinaryInterval]bool
}

func (s *realValueStore) beginIntervalRestore() func() error {
	frame := &intervalRestoreFrame{previous: s.intervalRestore, valueSet: make(map[*BinaryInterval]bool), extSet: make(map[*BinaryInterval]bool)}
	s.intervalRestore = frame
	return func() error {
		if s.intervalRestore != frame {
			return arena.ErrInvalidSlot
		}
		s.intervalRestore = frame.previous
		var err error
		for _, entry := range frame.values {
			err = errors.Join(err, entry.target.SetFrom(entry.saved), entry.saved.Close())
		}
		for _, entry := range frame.extensions {
			err = errors.Join(err, entry.target.SetFrom(entry.saved), entry.saved.Close())
		}
		return err
	}
}

func (s *realValueStore) intervalRestoreDefer(err *error) func() {
	restore := s.beginIntervalRestore()
	return func() { *err = errors.Join(*err, restore()) }
}

func (s *realValueStore) saveIntervalIfTooSmall(value *realValue, precision uint32) error {
	if s == nil || s.intervalRestore == nil || value == nil || precision <= s.intervalMaxPrecision() {
		return nil
	}
	interval, err := s.valueInterval(value, s.intervals)
	if err != nil {
		return err
	}
	contains, err := interval.ContainsZero()
	if err != nil || contains {
		return err
	}
	frame := s.intervalRestore
	if frame.valueSet[interval] {
		return nil
	}
	saved, err := copyBinaryInterval(interval)
	if err != nil {
		return err
	}
	frame.valueSet[interval] = true
	frame.values = append(frame.values, intervalRestoreEntry{target: interval, saved: saved})
	return nil
}

func (s *realValueStore) saveExtensionIntervalIfTooSmall(root *algebraicRoot, precision uint32) error {
	if s == nil || s.intervalRestore == nil || root == nil || root.interval == nil || precision <= s.intervalMaxPrecision() {
		return nil
	}
	contains, err := root.interval.ContainsZero()
	if err != nil || contains {
		return err
	}
	frame := s.intervalRestore
	if frame.extSet[root.interval] {
		return nil
	}
	saved, err := copyBinaryInterval(root.interval)
	if err != nil {
		return err
	}
	frame.extSet[root.interval] = true
	frame.extensions = append(frame.extensions, intervalRestoreEntry{target: root.interval, saved: saved})
	return nil
}

func copyBinaryInterval(source *BinaryInterval) (*BinaryInterval, error) {
	copy, err := NewBinaryInterval(source.numbers)
	if err != nil {
		return nil, err
	}
	if err = copy.SetFrom(source); err != nil {
		return nil, errors.Join(err, copy.Close())
	}
	return copy, nil
}
