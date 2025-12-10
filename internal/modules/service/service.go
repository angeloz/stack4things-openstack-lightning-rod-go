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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MDSLab/iotronic-lightning-rod/internal/board"
	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/MDSLab/iotronic-lightning-rod/internal/wamp"
	gammazero "github.com/gammazero/nexus/v3/client"
	nexuswamp "github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
)

// Manager handles service tunnel management via wstun
type Manager struct {
	mu sync.RWMutex

	board      *board.Board
	cfg        *config.Config
	wampClient *wamp.Client

	wstunIP   string
	wstunPort string
	wstunURL  string
	boardID   string

	services map[string]*ServiceInfo
}

// ServiceInfo represents a tunneled service
type ServiceInfo struct {
	Name      string `json:"name"`
	LocalPort int    `json:"local_port"`
	PublicURL string `json:"public_url"`
	PID       int    `json:"pid"`
	Status    string `json:"status"`
}

// ServicesConfig represents the services.json file
type ServicesConfig struct {
	Services map[string]*ServiceInfo `json:"services"`
}

// NewManager creates a new service manager
func NewManager(cfg *config.Config, board *board.Board, wampClient *wamp.Client) (*Manager, error) {
	m := &Manager{
		board:      board,
		cfg:        cfg,
		wampClient: wampClient,
		services:   make(map[string]*ServiceInfo),
		boardID:    board.UUID,
	}

	// Parse WAMP URL to get wstun connection info
	parsedURL, err := url.Parse(board.GetWampURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse WAMP URL: %w", err)
	}

	hostParts := strings.Split(parsedURL.Host, ":")
	m.wstunIP = hostParts[0]
	m.wstunPort = "8080" // Default wstun port

	// Determine protocol (ws or wss)
	protocol := "ws"
	if parsedURL.Scheme == "wss" {
		protocol = "wss"
	}
	m.wstunURL = fmt.Sprintf("%s://%s:%s", protocol, m.wstunIP, m.wstunPort)

	log.Infof("WSTUN bin path: %s", cfg.Services.WstunBin)
	log.Infof("WSTUN URL: %s", m.wstunURL)

	return m, nil
}

// Start initializes the service manager
func (m *Manager) Start(ctx context.Context) error {
	log.Info("Starting Service Manager...")

	// Load existing services configuration
	if err := m.loadServicesConfig(); err != nil {
		log.Warnf("Failed to load services config: %v", err)
	}

	// Register RPC procedures
	if err := m.registerRPCs(); err != nil {
		return fmt.Errorf("failed to register RPCs: %w", err)
	}

	log.Info("Service Manager started successfully")
	return nil
}

// Stop shuts down the service manager
func (m *Manager) Stop() error {
	log.Info("Stopping Service Manager...")

	// Stop all running services
	m.mu.Lock()
	defer m.mu.Unlock()

	for name := range m.services {
		if err := m.stopService(name); err != nil {
			log.Errorf("Failed to stop service %s: %v", name, err)
		}
	}

	return nil
}

// loadServicesConfig loads the services configuration from file
func (m *Manager) loadServicesConfig() error {
	configPath := filepath.Join(m.cfg.LightningRod.Home, "services.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create empty config
			return m.saveServicesConfig()
		}
		return err
	}

	var cfg ServicesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.services = cfg.Services
	if m.services == nil {
		m.services = make(map[string]*ServiceInfo)
	}

	return nil
}

// saveServicesConfig saves the services configuration to file
func (m *Manager) saveServicesConfig() error {
	configPath := filepath.Join(m.cfg.LightningRod.Home, "services.json")

	cfg := ServicesConfig{
		Services: m.services,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// registerRPCs registers service-related RPC procedures
func (m *Manager) registerRPCs() error {
	procedures := map[string]func(context.Context, *nexuswamp.Invocation) gammazero.InvokeResult{
		fmt.Sprintf("iotronic.%s.%s.ExposeService", m.board.SessionID, m.board.UUID):   m.handleExposeService,
		fmt.Sprintf("iotronic.%s.%s.UnexposeService", m.board.SessionID, m.board.UUID): m.handleUnexposeService,
		fmt.Sprintf("iotronic.%s.%s.ServicesList", m.board.SessionID, m.board.UUID):    m.handleServicesList,
	}

	for proc, handler := range procedures {
		if err := m.wampClient.Register(proc, handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", proc, err)
		}
		log.Infof("Registered RPC: %s", proc)
	}

	return nil
}

// handleExposeService handles the ExposeService RPC
func (m *Manager) handleExposeService(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC ExposeService called")

	if len(inv.Arguments) < 2 {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Missing arguments: service_name and local_port required",
			}},
		}
	}

	serviceName, ok := inv.Arguments[0].(string)
	if !ok {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Invalid service_name type",
			}},
		}
	}

	localPort, ok := inv.Arguments[1].(float64)
	if !ok {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Invalid local_port type",
			}},
		}
	}

	if err := m.exposeService(serviceName, int(localPort)); err != nil {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": fmt.Sprintf("Failed to expose service: %v", err),
			}},
		}
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": fmt.Sprintf("Service %s exposed on port %d", serviceName, int(localPort)),
		}},
	}
}

// handleUnexposeService handles the UnexposeService RPC
func (m *Manager) handleUnexposeService(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC UnexposeService called")

	if len(inv.Arguments) < 1 {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Missing argument: service_name required",
			}},
		}
	}

	serviceName, ok := inv.Arguments[0].(string)
	if !ok {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Invalid service_name type",
			}},
		}
	}

	if err := m.unexposeService(serviceName); err != nil {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": fmt.Sprintf("Failed to unexpose service: %v", err),
			}},
		}
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": fmt.Sprintf("Service %s unexposed", serviceName),
		}},
	}
}

// handleServicesList handles the ServicesList RPC
func (m *Manager) handleServicesList(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC ServicesList called")

	m.mu.RLock()
	servicesList := make([]map[string]any, 0, len(m.services))
	for _, svc := range m.services {
		servicesList = append(servicesList, map[string]any{
			"name":       svc.Name,
			"local_port": svc.LocalPort,
			"public_url": svc.PublicURL,
			"status":     svc.Status,
		})
	}
	m.mu.RUnlock()

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":   "SUCCESS",
			"message":  "Services list retrieved",
			"services": servicesList,
		}},
	}
}

// exposeService exposes a service via wstun
func (m *Manager) exposeService(name string, localPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if service already exists
	if _, exists := m.services[name]; exists {
		return fmt.Errorf("service %s already exposed", name)
	}

	// Start wstun tunnel
	cmd := exec.Command(
		m.cfg.Services.WstunBin,
		"client",
		"-s", m.wstunURL,
		"-t", fmt.Sprintf("127.0.0.1:%d", localPort),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start wstun: %w", err)
	}

	publicURL := fmt.Sprintf("%s/%s", m.wstunURL, name)

	// Store service info
	m.services[name] = &ServiceInfo{
		Name:      name,
		LocalPort: localPort,
		PublicURL: publicURL,
		PID:       cmd.Process.Pid,
		Status:    "running",
	}

	// Save configuration
	if err := m.saveServicesConfig(); err != nil {
		log.Warnf("Failed to save services config: %v", err)
	}

	log.Infof("Service %s exposed on port %d (PID: %d)", name, localPort, cmd.Process.Pid)

	return nil
}

// unexposeService stops and removes a service tunnel
func (m *Manager) unexposeService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopService(name)
}

// stopService stops a running service (must be called with lock held)
func (m *Manager) stopService(name string) error {
	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("service %s not found", name)
	}

	// Kill the wstun process
	if svc.PID > 0 {
		process, err := os.FindProcess(svc.PID)
		if err == nil {
			if err := process.Kill(); err != nil {
				log.Warnf("Failed to kill process %d: %v", svc.PID, err)
			}
		}
	}

	// Remove from services map
	delete(m.services, name)

	// Save configuration
	if err := m.saveServicesConfig(); err != nil {
		log.Warnf("Failed to save services config: %v", err)
	}

	log.Infof("Service %s unexposed", name)

	return nil
}
