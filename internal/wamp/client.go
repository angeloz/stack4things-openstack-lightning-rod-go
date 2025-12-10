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

package wamp

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/MDSLab/iotronic-lightning-rod/internal/board"
	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/gammazero/nexus/v3/client"
	"github.com/gammazero/nexus/v3/wamp"
	log "github.com/sirupsen/logrus"
)

// Client represents a WAMP client connection
type Client struct {
	mu sync.RWMutex

	board  *board.Board
	cfg    *config.Config
	client *client.Client
	ctx    context.Context
	cancel context.CancelFunc

	connected   bool
	sessionID   wamp.ID
	reconnTimer *time.Timer
}

// NewClient creates a new WAMP client
func NewClient(cfg *config.Config, board *board.Board) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		board:  board,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect establishes a connection to the WAMP router
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	wampURL := c.board.GetWampURL()
	realm := c.board.GetWampRealm()

	if wampURL == "" || realm == "" {
		return fmt.Errorf("WAMP configuration not available")
	}

	log.Infof("Connecting to WAMP router: %s (realm: %s)", wampURL, realm)

	// Configure TLS if using wss://
	cfg := client.Config{
		Realm: realm,
	}

	if c.cfg.LightningRod.SkipCertVerify {
		cfg.TlsCfg = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Create client
	cl, err := client.ConnectNet(c.ctx, wampURL, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to WAMP router: %w", err)
	}

	c.client = cl
	c.sessionID = cl.ID()
	c.connected = true

	// Update board session ID
	c.board.SessionID = fmt.Sprintf("%d", c.sessionID)

	log.Infof("Connected to WAMP router (session ID: %d)", c.sessionID)

	return nil
}

// Disconnect closes the WAMP connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			log.Warnf("Error closing WAMP client: %v", err)
		}
		c.client = nil
	}

	c.connected = false
	log.Info("Disconnected from WAMP router")

	return nil
}

// Register registers an RPC procedure
func (c *Client) Register(procedure string, handler func(context.Context, *wamp.Invocation) client.InvokeResult) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.client == nil {
		return fmt.Errorf("not connected to WAMP router")
	}

	if err := c.client.Register(procedure, handler, nil); err != nil {
		return fmt.Errorf("failed to register procedure %s: %w", procedure, err)
	}

	log.Debugf("Registered RPC procedure: %s", procedure)
	return nil
}

// Unregister unregisters an RPC procedure
func (c *Client) Unregister(procedure string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.client == nil {
		return fmt.Errorf("not connected to WAMP router")
	}

	if err := c.client.Unregister(procedure); err != nil {
		return fmt.Errorf("failed to unregister procedure %s: %w", procedure, err)
	}

	log.Debugf("Unregistered RPC procedure: %s", procedure)
	return nil
}

// Subscribe subscribes to a topic
func (c *Client) Subscribe(topic string, handler func(*wamp.Event)) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.client == nil {
		return fmt.Errorf("not connected to WAMP router")
	}

	if err := c.client.Subscribe(topic, handler, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	log.Debugf("Subscribed to topic: %s", topic)
	return nil
}

// Publish publishes a message to a topic
func (c *Client) Publish(topic string, args []any, kwargs map[string]any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.client == nil {
		return fmt.Errorf("not connected to WAMP router")
	}

	opts := wamp.Dict{}
	if err := c.client.Publish(topic, opts, args, kwargs); err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", topic, err)
	}

	log.Debugf("Published to topic: %s", topic)
	return nil
}

// Call invokes a remote procedure
func (c *Client) Call(procedure string, args []any, kwargs map[string]any) (*wamp.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("not connected to WAMP router")
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	result, err := c.client.Call(ctx, procedure, nil, args, kwargs, "")
	if err != nil {
		return nil, fmt.Errorf("failed to call procedure %s: %w", procedure, err)
	}

	return result, nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetSessionID returns the current session ID
func (c *Client) GetSessionID() wamp.ID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// KeepAlive starts a keep-alive routine to monitor connection health
func (c *Client) KeepAlive(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.cfg.Autobahn.AliveTimer) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.IsConnected() {
				log.Warn("Connection lost, attempting to reconnect...")
				if err := c.Reconnect(); err != nil {
					log.Errorf("Reconnection failed: %v", err)
				}
			}
		}
	}
}

// Reconnect attempts to reconnect to the WAMP router
func (c *Client) Reconnect() error {
	log.Info("Attempting to reconnect to WAMP router...")

	if err := c.Disconnect(); err != nil {
		log.Warnf("Error during disconnect before reconnect: %v", err)
	}

	time.Sleep(time.Duration(c.cfg.Autobahn.ConnectionTimer) * time.Second)

	if err := c.Connect(); err != nil {
		return fmt.Errorf("reconnection failed: %w", err)
	}

	log.Info("Successfully reconnected to WAMP router")
	return nil
}

// Stop stops the WAMP client
func (c *Client) Stop() {
	c.cancel()
	c.Disconnect()
}
