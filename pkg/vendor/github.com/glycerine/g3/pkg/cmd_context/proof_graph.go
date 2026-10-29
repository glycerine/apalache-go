package cmd_context

// Source: src/cmd_context/basic_cmds.cpp get_proof_graph_cmd.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// WriteProofGraph writes the latest proof to the configured Graphviz file.
func (c *Context) WriteProofGraph() (result error) {
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if !state.Proof {
		return fmt.Errorf("proof construction is not enabled, use command (set-option :produce-proofs true)")
	}
	if !c.checked || c.status != lbool.False {
		return fmt.Errorf("proof is not available")
	}
	proof, err := c.checkSatResult().Proof()
	if err != nil {
		return err
	}
	if proof.IsNull() {
		return fmt.Errorf("proof is not available")
	}
	if state.WellSortedCheck {
		good, e := c.manager.IsWellSorted(proof, nil)
		if e != nil {
			return e
		}
		if !good {
			return fmt.Errorf("proof is not well sorted")
		}
	}
	file, err := os.Create(c.parameters.DotProofFile())
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	if err = c.manager.WriteProofDOT(file, proof); err != nil {
		return err
	}
	_, err = io.WriteString(file, "\n")
	return err
}
