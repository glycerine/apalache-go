package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Source: upolynomial.h:363. These are raw numerals: the two explicit DELs
// are reached only after mk_monic returns, never on an exceptional body exit.
type upolyBerlekampMonic struct {
	lc, inv                        unumeral.Int
	declared, endEntered, ownEnded bool
	sourceIndex, ownIndex          uint8
	ownStarted                     bool
	cause                          error
}

func (s *upolyBerlekampMonic) at(i uint8) *unumeral.Int {
	if i == 0 {
		return &s.lc
	}
	return &s.inv
}
func (n *upolyBerlekampNumeric) finishMonicEnd() error {
	s := &n.monic
	if e := n.sourceCause(); e != nil {
		return e
	}
	s.endEntered = true
	for s.sourceIndex < 2 {
		if _, e := upolynomialSourceNumeralDel(n.owner.m.config.Integer, s.at(s.sourceIndex)); e != nil {
			s.cause = e
			return e
		}
		s.sourceIndex++
	}
	return nil
}
func (q *upolyBerlekampMatrix) cleanupMonicOwned() error {
	s := &q.numeric.monic
	if !s.declared || s.ownEnded || s.sourceIndex == 2 {
		return nil
	}
	if !s.ownStarted {
		s.ownStarted = true
		s.ownIndex = s.sourceIndex
	}
	for s.ownIndex < 2 {
		if e := q.m.config.Integer.CleanupRawNumeral(s.at(s.ownIndex)); e != nil {
			return e
		}
		s.ownIndex++
	}
	s.ownEnded = true
	return nil
}
