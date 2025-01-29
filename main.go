// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/s0up4200/SRTran/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
