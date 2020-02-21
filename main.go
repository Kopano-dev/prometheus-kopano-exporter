/*
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * Copyright 2019 Kopano and its licensors
 */

package main

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
)

const (
	defaultBadRequestMessage  = "Invalid data received."
	defaultTeaPotMessage      = "Sorry - you are a teapot."
	defaultServerErrorMessage = "Sorry - something bad has happend."
)

var (
	collectormap sync.Map
)

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

    err = ioutil.WriteFile("/tmp/dat1", body, 0644)
	if err != nil {
		log.Println("cannot write to file");
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

	log.Println(program_name)

	for key, value := range stats {
		val, _ := value.(map[string]interface{})
		key = program_name + "_" + key

		switch val["mode"] {
		case "gauge":
		case "counter":
			metrictype := val["type"]
			if metrictype != "int" {
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
		default:
			log.Printf("metric '%s' without mode", key)
		}
	}

	// TODO: Validate JSON schema.
	fmt.Fprintf(rw, "collected")
}

func main() {
	fmt.Println("starting kopano prometheus exporter")


	// Start promethes metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":2112", nil))
	}()

	file := "/tmp/kopano-prometheus.sock"

    if err := os.RemoveAll(file); err != nil {
        log.Fatal(err)
    }

	// TODO: add healthcheck for docker
	http.HandleFunc("/", collectMetricsHandler)

	unixListener, err := net.Listen("unix", file)
	if err != nil {
		panic(err)
	}

	defer unixListener.Close()

	log.Fatal(http.Serve(unixListener, nil))
}
