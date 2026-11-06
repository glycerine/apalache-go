package shell

import (
	"io"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func RunDatalogFrontend(context *cmd_context.Context, path string, output io.Writer) error {
	frontend, err := NewDatalogFrontend(context, output)
	if err != nil {
		return err
	}
	return frontend.Run(path)
}
