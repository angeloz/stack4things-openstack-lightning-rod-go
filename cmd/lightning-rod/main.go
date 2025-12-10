// Copyright 2024 MDSLAB - University of Messina
// All Rights Reserved.
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/MDSLab/iotronic-lightning-rod/internal/lightningrod"
	log "github.com/sirupsen/logrus"
)

const (
	Version = "1.0.0"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "/etc/iotronic/iotronic.conf", "Path to configuration file")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("Lightning-rod (Go) version %s\n", Version)
		os.Exit(0)
	}

	// Setup logging
	setupLogging(*logLevel)

	// Print banner
	printBanner()

	log.Infof("Lightning-rod:")
	log.Infof(" - version: %s", Version)
	log.Infof(" - PID: %d", os.Getpid())
	log.Infof(" - Config: %s", *configPath)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Infof(" - Home: %s", cfg.LightningRod.Home)
	log.Infof(" - Log level: %s", cfg.LightningRod.LogLevel)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Lightning Rod instance
	lr, err := lightningrod.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create Lightning Rod: %v", err)
	}

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start Lightning Rod in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- lr.Start(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Info("Received shutdown signal, stopping Lightning Rod...")
		cancel()
		lr.Stop()
	case err := <-errChan:
		if err != nil {
			log.Fatalf("Lightning Rod error: %v", err)
		}
	}

	log.Info("Lightning Rod stopped")
}

func setupLogging(level string) {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	switch level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

func printBanner() {
	banner := `
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║        Stack4Things Lightning-rod (Go Edition)                ║
║        IoTronic Board-side Agent                              ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}
