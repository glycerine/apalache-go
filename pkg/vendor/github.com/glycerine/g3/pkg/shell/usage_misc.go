package shell

import "io"

func writeUsageMisc(out io.Writer) error {
	return writeShellText(out, `
Miscellaneous:
  -h, -?      prints this message.
  -version    prints version number of Z3.
  -v:level    be verbose, where <level> is the verbosity level.
  -nw         disable warning messages.
  -p          display Z3 global (and module) parameters.
  -pd         display Z3 global (and module) parameter descriptions.
  -pm:name    display Z3 module ('name') parameters.
  -pp:name    display Z3 parameter description, if 'name' is not provided, then all module names are listed.
  --          all remaining arguments are assumed to be part of the input file name. This option allows Z3 to read files with strange names such as: -foo.smt2.
`)
}
