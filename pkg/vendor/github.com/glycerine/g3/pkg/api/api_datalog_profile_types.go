package api

import "github.com/glycerine/g3/pkg/shell"

type DatalogRelationProfile = shell.DatalogRelationProfile
type DatalogProfile = shell.DatalogFrontendProfile

type DatalogProfileResult struct {
	Profile DatalogProfile
	Timings shell.DatalogFrontendTimings
	Output  string
}
