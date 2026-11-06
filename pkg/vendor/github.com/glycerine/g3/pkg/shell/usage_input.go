package shell

import "io"

func writeUsageInput(out io.Writer) error {
	return writeShellText(out, `
Input format:
  -smt        use parser for SMT input format.
  -smt2       use parser for SMT 2 input format.
  -dl         use parser for Datalog input format.
  -dimacs     use parser for DIMACS input format.
  -log        use parser for Z3 log input format.
  -in         read formula from standard input.
`)
}
