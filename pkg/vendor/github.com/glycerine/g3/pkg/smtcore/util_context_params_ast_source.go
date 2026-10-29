package smtcore

// Source: src/cmd_context/context_params.cpp:198-209;
// src/ast/ast.cpp:1297-1334; src/smt/params/qi_params.cpp:43-64.

import (
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Concrete call-local filename reader. Go strings retain their own bytes across
// a later ContextParams update while the AST constructor reads this snapshot.

type astTraceFilename struct {
	params    uparams.ParamsStringView
	owned     string
	ownedMode bool
}

func (n astTraceFilename) Byte(i uint64) (byte, error) {
	if !n.ownedMode {
		return n.params.Byte(i)
	}
	if i > uint64(len(n.owned)) {
		return 0, arena.ErrBounds
	}
	if i == uint64(len(n.owned)) {
		return 0, nil
	}
	return n.owned[i], nil
}
func (n astTraceFilename) Length() (uint64, error) {
	if !n.ownedMode {
		return n.params.Length()
	}
	if i := strings.IndexByte(n.owned, 0); i >= 0 {
		return uint64(i), nil
	}
	return uint64(len(n.owned)), nil
}

// MkASTManager returns the actual placement-new failed owner on constructor
// failure. The caller may abort that exact owner; no successful source setters
// run before the constructor truly completes.
func (s *ContextParams) MkASTManager(c ast.ASTManagerConfig) (*ast.ASTManager, error) {
	if e := s.sourceCallReady(); e != nil {
		return nil, e
	}
	if c.Symbols != s.context.Symbols() || c.Numerals != s.context.Numerals() {
		return nil, uparams.ErrParamsOwner
	}
	c.ProofMode = ast.ASTProofDisabled
	if s.state.Proof {
		c.ProofMode = ast.ASTProofEnabled
	}
	var filename ast.TraceFilename
	if s.state.Trace {
		n := astTraceFilename{owned: s.traceFile, ownedMode: true}
		filename = n
	}
	s.executing = true
	defer func() { s.executing = false }()
	m, e := ast.NewSourceASTManagerBorrowed(c, filename)
	if e != nil {
		return m, e
	}
	if s.state.SMTLib2Compliant {
		if e = m.EnableIntRealCoercions(false); e != nil {
			return m, e
		}
	}
	if s.state.DebugRefCount {
		if e = m.EnableDebugRefCount(); e != nil {
			return m, e
		}
	}
	return m, nil
}
