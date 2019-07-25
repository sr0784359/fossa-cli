package report

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/apex/log"
	"github.com/urfave/cli"

	"github.com/fossas/fossa-cli/api/fossa"
	"github.com/fossas/fossa-cli/cmd/fossa/display"
	"github.com/fossas/fossa-cli/cmd/fossa/flags"
	"github.com/fossas/fossa-cli/cmd/fossa/setup"
	"github.com/fossas/fossa-cli/config"
	"github.com/fossas/fossa-cli/errors"
)

const defaultLicenseReportTemplate = `# 3rd-Party Software License Notice
Generated by fossa-cli (https://github.com/fossas/fossa-cli).
This software includes the following software and licenses:
{{range $license, $deps := .}}
========================================================================
{{$license}}
========================================================================
The following software have components provided under the terms of this license:
{{range $i, $dep := $deps}}
- {{$dep.Project.Title}} (from {{$dep.Project.URL}})
{{- end}}
{{end}}
`

var licensesCmd = cli.Command{
	Name:  "licenses",
	Usage: "Generate licenses report",
	Flags: flags.WithGlobalFlags(flags.WithAPIFlags(flags.WithOptions(flags.WithReportTemplateFlags([]cli.Flag{
		cli.BoolFlag{Name: JSON, Usage: "format output as JSON"},
	})))),
	Action: licensesRun,
}

func licensesRun(ctx *cli.Context) (err error) {
	err = setup.SetContext(ctx, true)
	if err != nil {
		log.Fatalf("Could not initialize: %s", err.Error())
	}

	defer display.ClearProgress()
	display.InProgress(fmt.Sprint("Fetching License Information"))

	locator := fossa.Locator{
		Fetcher:  config.Fetcher(),
		Project:  config.Project(),
		Revision: config.Revision(),
	}
	revs, err := fossa.GetRevisionDependencies(locator, true)
	if err != nil {
		return errors.Wrapf(err, "Unable to find licenses for project %s:", locator)
	}

	if ctx.Bool(JSON) {
		output, err := json.Marshal(revs)
		if err != nil {
			return err
		}
		fmt.Println(string(output))
		return nil
	}

	// Dependencies can have duplicate license matches which must be checked
	// otherwise a dependency can appear multiple times under a single license.
	revisionsByLicense := make(map[string][]fossa.Revision)
	for _, rev := range revs {
		foundLicenses := make(map[string]bool)
		for _, license := range rev.Licenses {
			_, duplicate := foundLicenses[license.LicenseID]
			if !duplicate {
				foundLicenses[license.LicenseID] = true
				revisionsByLicense[license.LicenseID] = append(revisionsByLicense[license.LicenseID], rev)
			}

		}
	}

	if len(revisionsByLicense) == 0 {
		fmt.Printf("\nNo licenses were found for project %s", locator)
		return nil
	}

	// Sort the dependency lists alphabetically by Project Title.
	for license, depList := range revisionsByLicense {
		sort.Slice(depList[:], func(i, j int) bool {
			return depList[i].Project.Title < depList[j].Project.Title
		})
		revisionsByLicense[license] = depList
	}

	output, err := display.TemplateString(defaultLicenseReportTemplate, revisionsByLicense)
	fmt.Println(output)
	return nil
}
