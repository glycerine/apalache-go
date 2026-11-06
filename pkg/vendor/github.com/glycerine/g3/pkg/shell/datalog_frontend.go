package shell

// Source: src/shell/datalog_frontend.cpp retained file/directory dispatch,
// parsing, saturation timing, and frontend result lifecycle.

import (
	"errors"
	"io"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrDatalogFrontend = errors.New("shell: invalid Datalog frontend state")

type DatalogPredicateResult struct {
	Declaration ast.ASTHandle
	Name        string
	Status      lbool.LBool
}

type DatalogFrontend struct {
	context                             *cmd_context.Context
	output                              io.Writer
	path                                string
	loaded, saturated                   bool
	printTuples                         bool
	started, parseFinished, runFinished time.Time
	results                             []DatalogPredicateResult
	snapshots                           []cmd_context.FixedpointRelationSnapshot
}

func (f *DatalogFrontend) RelationSnapshots() ([]cmd_context.FixedpointRelationSnapshot, error) {
	if f == nil || !f.saturated {
		return nil, ErrDatalogFrontend
	}
	result := make([]cmd_context.FixedpointRelationSnapshot, len(f.snapshots))
	for i, snapshot := range f.snapshots {
		result[i].Declaration = snapshot.Declaration
		result[i].Facts = make([][]uint64, len(snapshot.Facts))
		for j := range snapshot.Facts {
			result[i].Facts[j] = append([]uint64(nil), snapshot.Facts[j]...)
		}
	}
	return result, nil
}

func NewDatalogFrontend(context *cmd_context.Context, output io.Writer) (*DatalogFrontend, error) {
	if context == nil || output == nil {
		return nil, ErrDatalogFrontend
	}
	if _, err := context.Manager(); err != nil {
		return nil, err
	}
	return &DatalogFrontend{context: context, output: output}, nil
}

func (f *DatalogFrontend) Results() ([]DatalogPredicateResult, error) {
	if f == nil || !f.saturated {
		return nil, ErrDatalogFrontend
	}
	return append([]DatalogPredicateResult(nil), f.results...), nil
}
