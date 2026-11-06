package prime

// Source: src/util/prime_generator.cpp:23-27,99-130; src/util/prime_generator.h:29-52.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrServiceOwner = errors.New("g3: invalid default prime service owner")

// Source g_prime_generator is constructed before ordinary utility services.
// The current Go port is single-goroutine, so the source's OpenMP critical
// section is unnecessary for this process binding. Its nil budget keeps the
// global cache outside caller-owned arena accounting.
var processDefault = newProcessDefaultService()

func newProcessDefaultService() *DefaultService {
	s, e := NewDefaultService(nil)
	if e != nil {
		panic(e)
	}
	return s
}

// BorrowProcessGenerator is the nonowning binding for the source default
// prime iterator and its polynomial factorization consumer.
func BorrowProcessGenerator() (*Generator, error) {
	return processDefault.BorrowGenerator()
}

// FinalizeProcessGenerator matches prime_iterator::finalize: the object
// remains published, but its cached primes are cleared.
func FinalizeProcessGenerator() error { return processDefault.Finalize() }

// The actual generator is eagerly constructed with this binding, before any
// memory initialization call. Finalize clears its slice and keeps its identity.
// The enclosing header is charged cold Go binding metadata, not a source malloc
// event. Process-global initialization remains a separate obligation.
type DefaultService struct {
	self      *DefaultService
	generator *Generator
	budget    *arena.Budget
	header    uint64
	closed    bool
}
type ServiceStats struct {
	OwnerBytes, TotalBytes uint64
	Generator              Stats
}

func NewDefaultService(b *arena.Budget) (*DefaultService, error) {
	n := uint64(reflect.TypeOf(DefaultService{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	g, e := NewGenerator(b)
	if e != nil {
		b.Release(n)
		return nil, e
	}
	s := &DefaultService{generator: g, budget: b, header: n}
	s.self = s
	return s, nil
}
func (s *DefaultService) open() error {
	if s == nil || s.self != s {
		return ErrServiceOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *DefaultService) Finalize() error {
	if e := s.open(); e != nil {
		return e
	}
	return s.generator.Finalize()
}
func (s *DefaultService) Stats() ServiceStats {
	if s.open() != nil {
		return ServiceStats{}
	}
	g := s.generator.Stats()
	return ServiceStats{s.header, s.header + g.TotalBytes, g}
}
func (s *DefaultService) Close() error {
	if s == nil || s.self != s {
		return ErrServiceOwner
	}
	if s.closed {
		return nil
	}
	if e := s.generator.Close(); e != nil {
		return e
	}
	s.budget.Release(s.header)
	s.closed = true
	return nil
}

// This is a nonowning, freely copyable source iterator value. Local construction
// does not read the unused default service or the supplied generator contents.
type ServiceIterator struct {
	service   *DefaultService
	generator *Generator
	index     uint32
	global    bool
}

func NewServiceIterator(s *DefaultService, g *Generator) (ServiceIterator, error) {
	if g != nil {
		return ServiceIterator{generator: g}, nil
	}
	if s == nil {
		s = processDefault
	}
	if e := s.open(); e != nil {
		return ServiceIterator{}, e
	}
	return ServiceIterator{service: s, generator: s.generator, global: true}, nil
}
func (it *ServiceIterator) Next() (uint64, error) {
	if it == nil {
		return 0, arena.ErrConfig
	}
	index := it.index
	it.index++
	if !it.global {
		return it.generator.At(index)
	}
	if e := it.service.open(); e != nil {
		return 0, e
	}
	return it.generator.At(index)
}

// BorrowGenerator is an explicit Go binding for the default pointer selected
// by prime_generator.cpp:99-107. It preserves the actual eager object, including
// the empty-slice state after Finalize; the caller does not own the generator.
func (s *DefaultService) BorrowGenerator() (*Generator, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.generator, nil
}
