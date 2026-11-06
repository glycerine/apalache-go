package shell

import "io"

func writeUsageParameters(out io.Writer) error {
	return writeShellText(out, `
Parameter setting:
Global and module parameters can be set in the command line.
  param_name=value              for setting global parameters.
  module_name.param_name=value  for setting module parameters.
Use 'z3 -p' for the complete list of global and module parameters.
`)
}
