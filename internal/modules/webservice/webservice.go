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

package webservice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/MDSLab/iotronic-lightning-rod/internal/board"
	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/MDSLab/iotronic-lightning-rod/internal/wamp"
	gammazero "github.com/gammazero/nexus/v3/client"
	nexuswamp "github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
)

const (
	nginxConfDir = "/etc/nginx/conf.d"
)

// Manager handles webservice reverse proxy management via nginx
type Manager struct {
	mu sync.RWMutex

	board      *board.Board
	cfg        *config.Config
	wampClient *wamp.Client

	proxyType  string
	webservices map[string]*WebServiceInfo
}

// WebServiceInfo represents a reverse-proxied webservice
type WebServiceInfo struct {
	Name      string `json:"name"`
	LocalPort int    `json:"local_port"`
	PublicPort int   `json:"public_port"`
	Domain    string `json:"domain"`
	Status    string `json:"status"`
}

// NewManager creates a new webservice manager
func NewManager(cfg *config.Config, board *board.Board, wampClient *wamp.Client) (*Manager, error) {
	m := &Manager{
		board:       board,
		cfg:         cfg,
		wampClient:  wampClient,
		proxyType:   cfg.WebServices.Proxy,
		webservices: make(map[string]*WebServiceInfo),
	}

	log.Infof("Proxy used: %s", m.proxyType)

	return m, nil
}

// Start initializes the webservice manager
func (m *Manager) Start(ctx context.Context) error {
	log.Info("Starting WebService Manager...")

	// Verify nginx is available
	if _, err := exec.LookPath("nginx"); err != nil {
		log.Warnf("nginx not found, webservice management will be limited: %v", err)
	}

	// Register RPC procedures
	if err := m.registerRPCs(); err != nil {
		return fmt.Errorf("failed to register RPCs: %w", err)
	}

	log.Info("WebService Manager started successfully")
	return nil
}

// Stop shuts down the webservice manager
func (m *Manager) Stop() error {
	log.Info("Stopping WebService Manager...")

	// Clean up all webservices
	m.mu.Lock()
	defer m.mu.Unlock()

	for name := range m.webservices {
		if err := m.removeWebService(name); err != nil {
			log.Errorf("Failed to remove webservice %s: %v", name, err)
		}
	}

	return nil
}

// registerRPCs registers webservice-related RPC procedures
func (m *Manager) registerRPCs() error {
	procedures := map[string]func(context.Context, *nexuswamp.Invocation) gammazero.InvokeResult{
		fmt.Sprintf("iotronic.%s.%s.EnableWebService", m.board.SessionID, m.board.UUID):  m.handleEnableWebService,
		fmt.Sprintf("iotronic.%s.%s.DisableWebService", m.board.SessionID, m.board.UUID): m.handleDisableWebService,
		fmt.Sprintf("iotronic.%s.%s.WebServicesList", m.board.SessionID, m.board.UUID):   m.handleWebServicesList,
		fmt.Sprintf("iotronic.%s.%s.ProxyInfo", m.board.SessionID, m.board.UUID):         m.handleProxyInfo,
	}

	for proc, handler := range procedures {
		if err := m.wampClient.Register(proc, handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", proc, err)
		}
		log.Infof("Registered RPC: %s", proc)
	}

	return nil
}

// handleEnableWebService handles the EnableWebService RPC
func (m *Manager) handleEnableWebService(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC EnableWebService called")

	if len(inv.Arguments) < 3 {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Missing arguments: name, local_port, and public_port required",
			}},
		}
	}

	name, _ := inv.Arguments[0].(string)
	localPort, _ := inv.Arguments[1].(float64)
	publicPort, _ := inv.Arguments[2].(float64)

	if err := m.enableWebService(name, int(localPort), int(publicPort)); err != nil {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": fmt.Sprintf("Failed to enable webservice: %v", err),
			}},
		}
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": fmt.Sprintf("Webservice %s enabled", name),
		}},
	}
}

// handleDisableWebService handles the DisableWebService RPC
func (m *Manager) handleDisableWebService(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC DisableWebService called")

	if len(inv.Arguments) < 1 {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": "Missing argument: name required",
			}},
		}
	}

	name, _ := inv.Arguments[0].(string)

	if err := m.disableWebService(name); err != nil {
		return gammazero.InvokeResult{
			Args: []any{map[string]any{
				"result":  "ERROR",
				"message": fmt.Sprintf("Failed to disable webservice: %v", err),
			}},
		}
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": fmt.Sprintf("Webservice %s disabled", name),
		}},
	}
}

// handleWebServicesList handles the WebServicesList RPC
func (m *Manager) handleWebServicesList(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC WebServicesList called")

	m.mu.RLock()
	list := make([]map[string]any, 0, len(m.webservices))
	for _, ws := range m.webservices {
		list = append(list, map[string]any{
			"name":        ws.Name,
			"local_port":  ws.LocalPort,
			"public_port": ws.PublicPort,
			"status":      ws.Status,
		})
	}
	m.mu.RUnlock()

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":      "SUCCESS",
			"message":     "Webservices list retrieved",
			"webservices": list,
		}},
	}
}

// handleProxyInfo handles the ProxyInfo RPC
func (m *Manager) handleProxyInfo(ctx context.Context, inv *nexuswamp.Invocation) gammazero.InvokeResult {
	log.Info("RPC ProxyInfo called")

	// Check nginx status
	status := "stopped"
	if m.isNginxRunning() {
		status = "running"
	}

	return gammazero.InvokeResult{
		Args: []any{map[string]any{
			"result":  "SUCCESS",
			"message": "Proxy info retrieved",
			"data": map[string]any{
				"type":   m.proxyType,
				"status": status,
			},
		}},
	}
}

// enableWebService enables a webservice via nginx reverse proxy
func (m *Manager) enableWebService(name string, localPort, publicPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.webservices[name]; exists {
		return fmt.Errorf("webservice %s already enabled", name)
	}

	// Create nginx configuration
	confPath := filepath.Join(nginxConfDir, fmt.Sprintf("lr_%s.conf", name))
	nginxConf := fmt.Sprintf(`
server {
    listen %d;
    server_name _;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`, publicPort, localPort)

	if err := os.WriteFile(confPath, []byte(nginxConf), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Reload nginx
	if err := m.reloadNginx(); err != nil {
		os.Remove(confPath)
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	// Store webservice info
	m.webservices[name] = &WebServiceInfo{
		Name:       name,
		LocalPort:  localPort,
		PublicPort: publicPort,
		Status:     "enabled",
	}

	log.Infof("Webservice %s enabled (local:%d -> public:%d)", name, localPort, publicPort)

	return nil
}

// disableWebService disables a webservice
func (m *Manager) disableWebService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.removeWebService(name)
}

// removeWebService removes a webservice (must be called with lock held)
func (m *Manager) removeWebService(name string) error {
	if _, exists := m.webservices[name]; !exists {
		return fmt.Errorf("webservice %s not found", name)
	}

	// Remove nginx configuration
	confPath := filepath.Join(nginxConfDir, fmt.Sprintf("lr_%s.conf", name))
	if err := os.Remove(confPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to remove nginx config: %v", err)
	}

	// Reload nginx
	if err := m.reloadNginx(); err != nil {
		log.Warnf("Failed to reload nginx: %v", err)
	}

	// Remove from map
	delete(m.webservices, name)

	log.Infof("Webservice %s disabled", name)

	return nil
}

// reloadNginx reloads the nginx configuration
func (m *Manager) reloadNginx() error {
	// Test nginx configuration first
	cmd := exec.Command("nginx", "-t")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nginx config test failed: %s", output)
	}

	// Reload nginx
	cmd = exec.Command("nginx", "-s", "reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nginx reload failed: %s", output)
	}

	return nil
}

// isNginxRunning checks if nginx is running
func (m *Manager) isNginxRunning() bool {
	cmd := exec.Command("pgrep", "nginx")
	return cmd.Run() == nil
}
