package spacer

// Source: src/muz/spacer/spacer_virtual_solver.h and
// src/muz/spacer/spacer_virtual_solver.cpp timed benchmark dumping/recheck.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type virtualSolverRecheck func(assertions, assumptions []ast.ASTHandle) (lbool.LBool, error)

// VirtualSolverDumpConfig is the concrete source spacer.vs.* diagnostic
// policy. OutputDirectory defaults to the current directory; the explicit
// field lets hermetic callers keep optional diagnostic artifacts isolated.
type VirtualSolverDumpConfig struct {
	Enabled         bool
	MinSeconds      float64
	Recheck         bool
	OutputDirectory string
	recheck         virtualSolverRecheck
}

func (c *VirtualSolverDumpConfig) validate() error {
	if c == nil {
		return ErrVirtualSolverOwner
	}
	if strings.IndexByte(c.OutputDirectory, 0) >= 0 {
		return fmt.Errorf("muz/spacer: invalid benchmark output directory")
	}
	if c.OutputDirectory == "" {
		c.OutputDirectory = "."
	}
	return nil
}

func (f *VirtualSolverFactory) UpdateDumpConfig(config VirtualSolverDumpConfig) error {
	if err := f.open(); err != nil {
		return err
	}
	if err := config.validate(); err != nil {
		return err
	}
	f.dump = config
	return nil
}

func benchmarkStatus(status lbool.LBool) string {
	switch status {
	case lbool.True:
		return "sat"
	case lbool.False:
		return "unsat"
	default:
		return "unknown"
	}
}

func writeBenchmarkCheck(out io.Writer, manager *ast.ASTManager, assumptions []ast.ASTHandle) error {
	if _, err := io.WriteString(out, "(check-sat"); err != nil {
		return err
	}
	for _, assumption := range assumptions {
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := ast.DisplayLegacySMT(out, manager, assumption, 0, nil); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func (s *VirtualSolver) dumpBenchmark(status lbool.LBool, assumptions []ast.ASTHandle, seconds float64) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	config := s.factory.dump
	name := "virt_solver"
	if s.name != "" {
		name += "_" + s.name
	}
	name += fmt.Sprintf("_%d.smt2", s.dumpCounter)
	s.dumpCounter++
	path := filepath.Join(config.OutputDirectory, name)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			resultErr = errors.Join(resultErr, out.Close())
		}
	}()
	if _, err = fmt.Fprintf(out, "(set-info :status %s)\n", benchmarkStatus(status)); err != nil {
		return err
	}
	if err = s.Solver.Display(out, assumptions); err != nil {
		return err
	}
	if err = writeBenchmarkCheck(out, s.factory.manager, assumptions); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "(exit)\n"); err != nil {
		return err
	}
	stats := statistics.New()
	if err = s.Solver.CollectStatistics(stats); err != nil {
		return err
	}
	stats.UpdateDouble("time", seconds)
	if err = stats.DisplaySMT2(out); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	closed = true
	if config.Recheck && config.recheck != nil {
		assertions, assertionErr := s.SolverAssertions()
		if assertionErr != nil {
			return assertionErr
		}
		if _, err = config.recheck(assertions, assumptions); err != nil {
			return err
		}
	}
	return nil
}

func (s *VirtualSolver) SolverAssertions() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	count, err := s.Solver.NumAssertions()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		result[index], err = s.Solver.Assertion(index)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
