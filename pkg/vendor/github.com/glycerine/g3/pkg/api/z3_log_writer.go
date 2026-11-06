package api

// Source: src/api/z3_logger.h; src/api/api_log.cpp.

import (
	"errors"
	"io"
)

var (
	ErrZ3LogClosed = errors.New("z3 log is closed")
	ErrZ3LogWriter = errors.New("z3 log requires a writer")
)

type z3LogFlusher interface {
	Flush() error
}

// Z3LogWriter emits the line-oriented format consumed by Z3Replayer. Each
// operation is flushed, matching the C++ logger's crash-friendly behavior.
type Z3LogWriter struct {
	writer       io.Writer
	closer       io.Closer
	closed       bool
	failed       error
	configIDs    map[*Config]uint64
	nextObjectID uint64
}

func NewZ3LogWriter(writer io.Writer) (*Z3LogWriter, error) {
	if writer == nil {
		return nil, ErrZ3LogWriter
	}
	return &Z3LogWriter{writer: writer}, nil
}

func newOwnedZ3LogWriter(writer io.Writer, closer io.Closer) (*Z3LogWriter, error) {
	log, err := NewZ3LogWriter(writer)
	if err != nil {
		return nil, err
	}
	log.closer = closer
	return log, nil
}

func (log *Z3LogWriter) writeRecord(record string) error {
	if log == nil || log.closed || log.writer == nil {
		return ErrZ3LogClosed
	}
	if log.failed != nil {
		return log.failed
	}
	n, err := io.WriteString(log.writer, record)
	if err == nil && n != len(record) {
		err = io.ErrShortWrite
	}
	if err == nil {
		if flusher, ok := log.writer.(z3LogFlusher); ok {
			err = flusher.Flush()
		}
	}
	if err != nil {
		log.failed = err
	}
	return err
}

func (log *Z3LogWriter) Err() error {
	if log == nil {
		return ErrZ3LogClosed
	}
	return log.failed
}

func (log *Z3LogWriter) Close() error {
	if log == nil || log.closed {
		return nil
	}
	log.closed = true
	err := log.failed
	if log.closer != nil {
		err = errors.Join(err, log.closer.Close())
	}
	log.writer, log.closer, log.configIDs = nil, nil, nil
	return err
}
