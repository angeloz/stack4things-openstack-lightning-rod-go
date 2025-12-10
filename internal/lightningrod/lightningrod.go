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

package lightningrod

import (
	"context"
	"fmt"
	"sync"

	"github.com/MDSLab/iotronic-lightning-rod/internal/board"
	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/MDSLab/iotronic-lightning-rod/internal/modules/device"
	"github.com/MDSLab/iotronic-lightning-rod/internal/modules/rest"
	"github.com/MDSLab/iotronic-lightning-rod/internal/modules/service"
	"github.com/MDSLab/iotronic-lightning-rod/internal/modules/webservice"
	"github.com/MDSLab/iotronic-lightning-rod/internal/wamp"
	log "github.com/sirupsen/logrus"
)

// LightningRod is the main application struct
type LightningRod struct {
	cfg    *config.Config
	board  *board.Board
	wamp   *wamp.Client
	rest   *rest.Manager
	device *device.Manager
	service *service.Manager
	webservice *webservice.Manager

	mu      sync.Mutex
	running bool
}

// New creates a new Lightning Rod instance
func New(cfg *config.Config) (*LightningRod, error) {
	// Create board instance
	board, err := board.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create board: %w", err)
	}

	lr := &LightningRod{
		cfg:   cfg,
		board: board,
	}

	// Initialize WAMP client
	lr.wamp = wamp.NewClient(cfg, board)

	// Initialize REST API manager (starts immediately, no WAMP dependency)
	restMgr, err := rest.NewManager(cfg, board)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST manager: %w", err)
	}
	lr.rest = restMgr

	return lr, nil
}

// Start starts the Lightning Rod
func (lr *LightningRod) Start(ctx context.Context) error {
	lr.mu.Lock()
	if lr.running {
		lr.mu.Unlock()
		return fmt.Errorf("lightning rod already running")
	}
	lr.running = true
	lr.mu.Unlock()

	log.Info("Starting Lightning Rod...")

	// Start REST API server
	if err := lr.rest.Start(ctx); err != nil {
		return fmt.Errorf("failed to start REST API: %w", err)
	}

	// Connect to WAMP router
	log.Info("Connecting to WAMP router...")
	if err := lr.wamp.Connect(); err != nil {
		return fmt.Errorf("failed to connect to WAMP router: %w", err)
	}

	// Initialize modules that depend on WAMP
	if err := lr.initializeModules(ctx); err != nil {
		return fmt.Errorf("failed to initialize modules: %w", err)
	}

	// Start keep-alive monitoring
	go lr.wamp.KeepAlive(ctx)

	log.Info("Lightning Rod started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	return nil
}

// initializeModules initializes all modules
func (lr *LightningRod) initializeModules(ctx context.Context) error {
	log.Info("Initializing modules...")

	// Initialize Device Manager
	deviceMgr, err := device.NewManager(lr.cfg, lr.board, lr.wamp)
	if err != nil {
		return fmt.Errorf("failed to create device manager: %w", err)
	}
	lr.device = deviceMgr

	if err := lr.device.Start(ctx); err != nil {
		return fmt.Errorf("failed to start device manager: %w", err)
	}

	// Initialize Service Manager
	serviceMgr, err := service.NewManager(lr.cfg, lr.board, lr.wamp)
	if err != nil {
		return fmt.Errorf("failed to create service manager: %w", err)
	}
	lr.service = serviceMgr

	if err := lr.service.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service manager: %w", err)
	}

	// Initialize WebService Manager
	webserviceMgr, err := webservice.NewManager(lr.cfg, lr.board, lr.wamp)
	if err != nil {
		return fmt.Errorf("failed to create webservice manager: %w", err)
	}
	lr.webservice = webserviceMgr

	if err := lr.webservice.Start(ctx); err != nil {
		return fmt.Errorf("failed to start webservice manager: %w", err)
	}

	log.Info("All modules initialized successfully")

	return nil
}

// Stop stops the Lightning Rod
func (lr *LightningRod) Stop() {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if !lr.running {
		return
	}

	log.Info("Stopping Lightning Rod...")

	// Stop modules in reverse order
	if lr.webservice != nil {
		if err := lr.webservice.Stop(); err != nil {
			log.Errorf("Error stopping webservice manager: %v", err)
		}
	}

	if lr.service != nil {
		if err := lr.service.Stop(); err != nil {
			log.Errorf("Error stopping service manager: %v", err)
		}
	}

	if lr.device != nil {
		if err := lr.device.Stop(); err != nil {
			log.Errorf("Error stopping device manager: %v", err)
		}
	}

	// Stop WAMP connection
	if lr.wamp != nil {
		lr.wamp.Stop()
	}

	// Stop REST API
	if lr.rest != nil {
		if err := lr.rest.Stop(); err != nil {
			log.Errorf("Error stopping REST API: %v", err)
		}
	}

	lr.running = false
	log.Info("Lightning Rod stopped")
}
