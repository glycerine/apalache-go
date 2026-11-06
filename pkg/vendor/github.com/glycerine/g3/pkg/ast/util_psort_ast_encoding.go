package ast

// Source: src/util/sorting_network.h sorting_network_encoding.

type PsortASTEncoding uint8

const (
	PsortASTGrouped PsortASTEncoding = iota
	PsortASTBimander
	PsortASTOrdered
	PsortASTUnate
	PsortASTCircuit
)

func (s *PsortASTNetwork) SetEncoding(encoding PsortASTEncoding) error {
	if e := s.open(); e != nil {
		return e
	}
	if encoding > PsortASTCircuit {
		return ErrASTDomain
	}
	s.encoding = encoding
	return nil
}

func (s *PsortASTNetwork) Encoding() (PsortASTEncoding, error) {
	if e := s.open(); e != nil {
		return PsortASTGrouped, e
	}
	return s.encoding, nil
}
