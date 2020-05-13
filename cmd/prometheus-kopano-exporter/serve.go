/*
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * Copyright 2020 Kopano and its licensors
 */

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"stash.kopano.io/kc/prometheus-kopano-exporter/server"
)

var (
	listensocket  = "/run/prometheus-kopano-exporter/exporter.sock"
	listenaddress = "localhost:6231"
)

func commandServe() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Prometheus exporter for Kopano server, dagent and spooler",
		Run: func(cmd *cobra.Command, args []string) {
			if err := serve(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	serveCmd.Flags().StringVar(&listensocket, "listen-socket", listensocket, "Custom socket location")
	serveCmd.Flags().StringVar(&listenaddress, "listen-address", listenaddress, "Address on which to expose metrics and web interface.")

	serveCmd.Flags().Bool("log-timestamp", true, "Prefix each log line with timestamp")
	serveCmd.Flags().String("log-level", "info", "Log level (one of panic, fatal, error, warn, info or debug)")

	return serveCmd
}

func serve(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logTimestamp, _ := cmd.Flags().GetBool("log-timestamp")
	logLevel, _ := cmd.Flags().GetString("log-level")

	logger, err := newLogger(!logTimestamp, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	logger.Debugln("starting kopano prometheus exporter")

	cfg := &server.Config{
		ListenSocket:  listensocket,
		ListenAddress: listenaddress,

		Logger: logger,
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		return err
	}

	return srv.Serve(ctx)
}
