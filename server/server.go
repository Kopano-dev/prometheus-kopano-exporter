/*
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * Copyright 2020 Kopano and its licensors
 */
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

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

// Server is our HTTP server implementation.
type Server struct {
	config *Config

	logger logrus.FieldLogger
}

func NewServer(c *Config) (*Server, error) {
	s := &Server{
		config: c,
		logger: c.Logger,
	}

	return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	var err error
	var wg sync.WaitGroup

	_, serveCtxCancel := context.WithCancel(ctx)
	defer serveCtxCancel()

	logger := s.logger

	if err := os.RemoveAll(s.config.ListenSocket); err != nil {
		logger.WithError(err).Errorf("failed to remove socket location")
		return err
	}

	unixListener, err := net.Listen("unix", s.config.ListenSocket)
	if err != nil {
		logger.WithError(err).Errorf("failed to create socket")
		return err
	}
	defer unixListener.Close()

	listener, err := net.Listen("tcp", s.config.ListenAddress)
	if err != nil {
		logger.WithError(err).Errorf("failed to create http socket")
		return err
	}
	defer listener.Close()

	promServer := http.Server{
		Handler: promhttp.Handler(),
	}

	unixHandler := http.HandlerFunc(s.collectMetricsHandler)
	unixServer := http.Server{
		Handler: unixHandler,
	}

	errCh := make(chan error, 2)
	exitCh := make(chan bool, 1)
	signalCh := make(chan os.Signal, 1)

	wg.Add(1)

	// Start Prometheus metrics
	go func() {
		defer wg.Done()

		err := promServer.Serve(listener)
		if err != nil {
			errCh <- err
		}

		logger.Debugln("http listener stopped")
	}()

	wg.Add(1)

	// Start metrics endpoint
	go func() {
		defer wg.Done()

		err = unixServer.Serve(unixListener)
		if err != nil {
			errCh <- err
		}

		logger.Debugln("unix listener stopped")
	}()

	// Wait for exit or error.
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		// breaks
	case reason := <-signalCh:
		logger.WithField("signal", reason).Warnln("received signal")
		// breaks
	}

	logger.Infoln("clean server shutdown start")

	shutDownCtx, shutDownCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	go func() {
		if shutdownErr := promServer.Shutdown(shutDownCtx); shutdownErr != nil {
			logger.WithError(shutdownErr).Warn("clean http server shutdown failed")
		}
	}()

	shutDownCtx2, shutDownCtxCancel2 := context.WithTimeout(ctx, 10*time.Second)
	go func() {
		if shutdownErr := unixServer.Shutdown(shutDownCtx2); shutdownErr != nil {
			logger.WithError(shutdownErr).Warn("clean unix server shutdown failed")
		}
	}()

	go func() {
		wg.Wait()
		close(exitCh)
	}()
    shutDownCtxCancel()  // prevent leak.
    shutDownCtxCancel2() // prevent leak.

	// Cancel our own context,
	serveCtxCancel()
	func() {
		for {
			select {
			case <-exitCh:
				return
			default:
				// Unix/HTTP listener has not quit yet.
				logger.Info("waiting for listeners to exit")
			}
			select {
			case reason := <-signalCh:
				logger.WithField("signal", reason).Warn("received signal")
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()

	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}

	return err
}

func (s *Server) collectMetricsHandler(rw http.ResponseWriter, req *http.Request) {
	logger := s.logger

	if req.Header.Get("X-Kopano-Stats-Request") != "1" {
		WriteBadRequestPage(rw, "missing header")
		return
	}

	if req.Method != http.MethodPost {
		WriteBadRequestPage(rw, "request must use the POST method")
		return
	}

	if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
		WriteBadRequestPage(rw, "json content type")
		return
	}

	body, err := ioutil.ReadAll(http.MaxBytesReader(rw, req.Body, 1*1024*1024))
	if err != nil {
		logger.WithError(err).Errorf("failed to read client data request")
		fmt.Fprintln(rw, "failed to read client data request")
		WriteBadRequestPage(rw, "")
		return
	}

	// Parse JSON.
	payload := make(map[string]interface{})
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Fprintln(rw, "failed to parse json")
		logger.WithError(err).Errorf("failed to parse JSON")
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

	logger.WithField("program", program_name).Info("receiving metrics")

	for key, value := range stats {
		val, _ := value.(map[string]interface{})

		if strings.HasPrefix(key, strip_string) {
			key = strings.Replace(key, strip_string, "", 1)
		}

		key = program_name + "_" + key
		mode := val["mode"]

		switch mode {
		case "gauge", "counter":
			metrictype := val["type"]
			if metrictype != "int" {
				logger.WithField("metrictype", key).Debug("skipping non integer metric type")
				break
			}

			gauge, ok := collectormap.Load(key)
			if !ok {
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
			if mode != nil {
				logger.WithField("mode", mode).Debug("unsupported metric mode")
			} else {
				logger.WithField("key", key).Debug("metric has no mode")
			}
		}
	}

	// TODO: Validate JSON schema.
	fmt.Fprintf(rw, "collected")
}

// TODO: add a more useful health report
func (s *Server) healthCheckHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

func WriteBadRequestPage(rw http.ResponseWriter, message string) {
	if message == "" {
		message = defaultBadRequestMessage
	}
	http.Error(rw, message, http.StatusBadRequest)
}
