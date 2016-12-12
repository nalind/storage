package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/storage/pkg/mflag"
	"github.com/containers/storage/storage"
)

func list_drivers(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	names := storage.ListGraphDrivers()
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(names)
	} else {
		for _, name := range names {
			fmt.Fprintf(os.Stderr, "%s\n", name)
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:   []string{"drivers"},
		usage:   "List the registered drivers",
		minArgs: 0,
		action:  list_drivers,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
		},
	})
}
