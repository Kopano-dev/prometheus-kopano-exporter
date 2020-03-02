/*
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * Copyright 2020 Kopano and its licensors
 */

package main

import (
	"stash.kopano.com/kgol/prometheus-kopano-exporter/cmd"
)

func main() {
	cmd.RootCmd.AddCommand(commandServe())
	cmd.RootCmd.Execute()
}
