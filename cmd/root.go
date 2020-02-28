/*
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * Copyright 2019 Kopano and its licensors
 */
package cmd

import (
	"fmt"
	"encoding/json"
	"net"
	"net/http"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"
)

const (
	defaultBadRequestMessage  = "Invalid data received."
	defaultTeaPotMessage      = "Sorry - you are a teapot."
	defaultServerErrorMessage = "Sorry - something bad has happend."
)

var (
	collectormap sync.Map
	socketloc = "/tmp/kopano-prometheus.sock"
	httpport = ":9099"
)

var rootCmd = &cobra.Command{
  Use:   "prometheus-kopano-exporter",
  Short: "Prometheus exporter for Kopano server, dagent and spooler",
  Run: func(cmd *cobra.Command, args []string) {
    // Do Stuff Here
	fmt.Println("starting kopano prometheus exporter")


	// Start promethes metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(httpport, nil))
	}()

    if err := os.RemoveAll(socketloc); err != nil {
        log.Fatal(err)
    }

	// TODO: add healthcheck for docker
	http.HandleFunc("/", collectMetricsHandler)

	unixListener, err := net.Listen("unix", socketloc)
	if err != nil {
		panic(err)
	}

	defer unixListener.Close()

	log.Fatal(http.Serve(unixListener, nil))
  },
}

func WriteBadRequestPage(rw http.ResponseWriter, message string) {
	if message == "" {
		message = defaultBadRequestMessage
	}
		http.Error(rw, message, http.StatusBadRequest)
}

func collectMetricsHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-Kopano-Stats-Request") != "1" {
		WriteBadRequestPage(rw, "missing header")
		return
	}

	if req.Method != http.MethodPost {
		WriteBadRequestPage(rw, "not a post")
		return
	}

	if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
		WriteBadRequestPage(rw, "json content type")
		return
	}

	body, err := ioutil.ReadAll(http.MaxBytesReader(rw, req.Body, 1*1024*1024))
	if err != nil {
		// TODO: logger
		fmt.Fprintln(rw, "failed to read client data request")
		WriteBadRequestPage(rw, "")
		return
	}

	// Parse JSON.
	payload := make(map[string]interface{})
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Fprintln(rw, "failed to parse json")
		WriteBadRequestPage(rw, "")
		return
	}

	// Iterate over stats and add counters
	stats, ok := payload["stats"].(map[string]interface{})
	if !ok {
		WriteBadRequestPage(rw, "")
		return
	}

	program_key := stats["program_name"].(map[string]interface{})
	program_name := strings.ReplaceAll(program_key["value"].(string), "-", "_")

	// Some metrics are prefixed with the program name such as 'dagent_deliver_junk'
	strip_string := ""
	if strings.HasPrefix(program_name, "kopano_") {
		// Use split?
		strip_string = strings.Replace(program_name, "kopano_", "", 1)
		strip_string += "_"
	}

	log.Printf("Receiving metrics from %s", program_name)

	for key, value := range stats {
		val, _ := value.(map[string]interface{})

		if strings.HasPrefix(key, strip_string) {
			key = strings.Replace(key, strip_string, "", 1)
		}

		key = program_name + "_" + key

		switch val["mode"] {
		case "gauge", "counter":
			metrictype := val["type"]
			if metrictype != "int" {
				log.Printf("skipping metric '%s' as it's not of type integer", key)
				break
			}

			gauge, ok := collectormap.Load(key)
			if ! ok {
				gauge = promauto.NewGauge(prometheus.GaugeOpts{
						Name: key,
						Help: val["desc"].(string),
				})
				collectormap.Store(key, gauge)
			}

			gauge.(prometheus.Gauge).Set(val["value"].(float64))
			break
		case "unixtime":
			break
		default:
			log.Printf("metric '%s' without mode", key)
		}
	}

	// TODO: Validate JSON schema.
	fmt.Fprintf(rw, "collected")
}

func Execute() {
  rootCmd.Flags().StringVar(&socketloc, "socket-loc", socketloc, "Custom socket location")
  rootCmd.Flags().StringVar(&httpport, "listen-address", httpport, "Address on which to expose metrics and web interface.")

  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}
