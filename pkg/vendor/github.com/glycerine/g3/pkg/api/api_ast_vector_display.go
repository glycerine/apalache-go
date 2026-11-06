package api

func (v *ASTVector) String() (string, error) {
	if err := v.check(); err != nil {
		return "", err
	}
	return v.object.string()
}
