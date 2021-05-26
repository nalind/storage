package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
	multierror "github.com/hashicorp/go-multierror"
)

var (
	repair, forceRepair bool
)

func check(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	checker, ok := m.(interface {
		Check(*storage.CheckOptions) (storage.CheckReport, error)
	})
	if !ok {
		fmt.Fprintf(os.Stderr, "check not implemented\n")
		return 1
	}
	report, err := checker.Check(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: %v\n", err)
		return 1
	}
	outputNonJSON := func(report storage.CheckReport) {
		for id, errs := range report.Layers {
			if len(errs) > 0 {
				fmt.Fprintf(os.Stdout, "layer %s:\n", id)
			}
			for _, err := range errs {
				fmt.Fprintf(os.Stdout, " %v\n", err)
			}
		}
		for id, errs := range report.Images {
			if len(errs) > 0 {
				fmt.Fprintf(os.Stdout, "image %s:\n", id)
			}
			for _, err := range errs {
				fmt.Fprintf(os.Stdout, " %v\n", err)
			}
		}
		for id, errs := range report.Containers {
			if len(errs) > 0 {
				fmt.Fprintf(os.Stdout, "container %s:\n", id)
			}
			for _, err := range errs {
				fmt.Fprintf(os.Stdout, " %v\n", err)
			}
		}
	}
	if !repair {
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(report)
		} else {
			outputNonJSON(report)
		}
		if len(report.Layers) > 0 || len(report.Images) > 0 || len(report.Containers) > 0 {
			return 1
		}
		return 0
	}
	repairer, ok := m.(interface {
		Repair(storage.CheckReport, *storage.RepairOptions) []error
	})
	if !ok {
		fmt.Fprintf(os.Stderr, "repair not implemented\n")
		return 1
	}
	if errs := repairer.Repair(report, nil); errs != nil {
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(errs)
		} else {
			fmt.Fprintf(os.Stderr, "%v", multierror.Append(nil, errs...))
		}
		return 1
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:   []string{"check"},
		usage:   "Check storage consistency",
		minArgs: 0,
		maxArgs: 0,
		action:  check,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			flags.BoolVar(&repair, []string{"-repair", "r"}, repair, "Remove damaged images and layers")
			flags.BoolVar(&forceRepair, []string{"-force", "f"}, forceRepair, "Remove damaged containers")
		},
	})
}
