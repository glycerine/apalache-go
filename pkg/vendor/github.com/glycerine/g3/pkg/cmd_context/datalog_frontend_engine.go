package cmd_context

// SelectDatalogFrontendEngine follows read_datalog's local engine=datalog
// selection before parsing, without replacing module or other local options.
func (c *Context) SelectDatalogFrontendEngine() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.SelectEngine("datalog")
}
