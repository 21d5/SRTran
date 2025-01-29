// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version information
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of SRTran",
	Long:  `All software has versions. This is SRTran's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("SRTran %s\n", Version)
		if verbose {
			fmt.Printf("Build Time: %s\n", BuildTime)
			fmt.Printf("Git Commit: %s\n", GitCommit)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
