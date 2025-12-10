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

package device

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MDSLab/iotronic-lightning-rod/internal/board"
	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/MDSLab/iotronic-lightning-rod/internal/wamp"
	gammazero "github.com/gammazero/nexus/v3/client"
	nexuswamp "github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
)

// Manager handles device-specific operations
type Manager struct {
	board      *board.Board
	cfg        *config.Config
	wampClient *wamp.Client
	device     Device
}

// Device interface for device-specific implementations
type Device interface {
	GetType() string
	GetInfo() (map[string]any, error)
	GetStatus() (map[string]any, error)
}

// GenericDevice represents a generic device implementation
type GenericDevice struct {
	deviceType string
}

// NewManager creates a new device manager
func NewManager(cfg *config.Config, board *board.Board, wampClient *wamp.Client) (*Manager, error) {
	m := &Manager{
		board:      board,
		cfg:        cfg,
		wampClient: wampClient,
	}

	// Initialize device based on board type
	m.device = &GenericDevice{deviceType: board.Type}

	log.Infof("Device Manager initialized for type: %s", board.Type)

	return m, nil
}

// Start initializes the device manager
func (m *Manager) Start(ctx context.Context) error {
	log.Info("Starting Device Manager...")

	// Register RPC procedures
	if err := m.registerRPCs(); err != nil {
		return fmt.Errorf("failed to register RPCs: %w", err)
	}

	log.Info("Device Manager started successfully")
	return nil
}

// Stop shuts down the device manager
func (m *Manager) Stop() error {
	log.Info("Stopping Device Manager...")
	return nil
}

// registerRPCs registers device-related RPC procedures
func (m *Manager) registerRPCs() error {
	procedures := map[string]func(context.Context, *nexuswamp.Invocation) gammazero.InvokeResult{
		fmt.Sprintf("iotronic.%s.%s.DevicePing", m.board.SessionID, m.board.UUID):   m.handleDevicePing,
		fmt.Sprintf("iotronic.%s.%s.DeviceInfo", m.board.SessionID, m.board.UUID):   m.handleDeviceInfo,
		fmt.Sprintf("iotronic.%s.%s.DeviceStatus", m.board.SessionID, m.board.UUID): m.handleDeviceStatus,
	}

	for proc, handler := range procedures {
		if err := m.wampClient.Register(proc, handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", proc, err)
		}
		log.Infof("Registered RPC: %s", proc)
	}

	return nil
}

// handleDevicePing handles the DevicePing RPC
func (m *Manager) handleDevicePing(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC DevicePing called")

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	message := fmt.Sprintf("%s @ %s", hostname, time.Now().Format("2006-01-02T15:04:05.000000"))

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": message,
		}},
	}
}

// handleDeviceInfo handles the DeviceInfo RPC
func (m *Manager) handleDeviceInfo(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC DeviceInfo called")

	info, err := m.device.GetInfo()
	if err != nil {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": err.Error(),
			}},
		}
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": "Device info retrieved",
			"data":    info,
		}},
	}
}

// handleDeviceStatus handles the DeviceStatus RPC
func (m *Manager) handleDeviceStatus(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC DeviceStatus called")

	status, err := m.device.GetStatus()
	if err != nil {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": err.Error(),
			}},
		}
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": "Device status retrieved",
			"data":    status,
		}},
	}
}

// GenericDevice implementation

func (d *GenericDevice) GetType() string {
	return d.deviceType
}

func (d *GenericDevice) GetInfo() (map[string]any, error) {
	hostname, _ := os.Hostname()

	return map[string]any{
		"type":     d.deviceType,
		"hostname": hostname,
	}, nil
}

func (d *GenericDevice) GetStatus() (map[string]any, error) {
	return map[string]any{
		"status": "online",
		"uptime": time.Now().Unix(),
	}, nil
}
