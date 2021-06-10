/*
Copyright 2019 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package cli

var (
	cli_name    = "unknown"
	cli_version = "unknown"
	cli_commit  = "unknown"
	cli_dirty   = ""
)

type CompiledEnv struct {
	Name    string
	Version string
	Commit  string
	Dirty   bool
}

var env CompiledEnv

func init() {
	// must be created inside the init function to pickup build specific params
	env = CompiledEnv{
		Name:    cli_name,
		Version: cli_version,
		Commit:  cli_commit,
		Dirty:   cli_dirty != "",
	}
}
