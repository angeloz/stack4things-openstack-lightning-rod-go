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

package rest

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/MDSLab/iotronic-lightning-rod/internal/board"
	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	log "github.com/sirupsen/logrus"
)

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var static embed.FS

const defaultPort = "8080"

// Manager handles the REST API server
type Manager struct {
	board  *board.Board
	cfg    *config.Config
	server *http.Server
	router *gin.Engine
}

// NewManager creates a new REST manager
func NewManager(cfg *config.Config, board *board.Board) (*Manager, error) {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	m := &Manager{
		board:  board,
		cfg:    cfg,
		router: gin.New(),
	}

	// Setup middleware
	m.router.Use(gin.Recovery())
	m.router.Use(m.loggerMiddleware())

	// Setup routes
	m.setupRoutes()

	return m, nil
}

// Start starts the REST API server
func (m *Manager) Start(ctx context.Context) error {
	log.Info("Starting REST API server...")

	port := defaultPort
	addr := fmt.Sprintf(":%s", port)

	m.server = &http.Server{
		Addr:    addr,
		Handler: m.router,
	}

	// Start server in goroutine
	go func() {
		log.Infof("REST API server listening on %s", addr)
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("REST API server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the REST API server
func (m *Manager) Stop() error {
	log.Info("Stopping REST API server...")

	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	return nil
}

// setupRoutes configures all HTTP routes
func (m *Manager) setupRoutes() {
	// Static files
	m.router.StaticFS("/static", http.FS(static))

	// API routes
	api := m.router.Group("/api")
	{
		api.GET("/info", m.handleInfo)
		api.GET("/status", m.handleStatus)
		api.GET("/board", m.handleBoard)
	}

	// Web UI routes
	m.router.GET("/", m.handleHome)
	m.router.GET("/dashboard", m.handleDashboard)
}

// loggerMiddleware provides request logging
func (m *Manager) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		log.Debugf("%s %s - %d (%v)",
			c.Request.Method,
			path,
			c.Writer.Status(),
			duration,
		)
	}
}

// handleInfo returns Lightning Rod information
func (m *Manager) handleInfo(c *gin.Context) {
	hostname, _ := os.Hostname()

	c.JSON(http.StatusOK, gin.H{
		"name":    "Lightning-rod",
		"version": "1.0.0",
		"board": gin.H{
			"uuid":     m.board.UUID,
			"name":     m.board.Name,
			"type":     m.board.Type,
			"status":   m.board.Status,
			"hostname": hostname,
		},
		"wamp": gin.H{
			"connected":  true,
			"session_id": m.board.SessionID,
			"url":        m.board.GetWampURL(),
			"realm":      m.board.GetWampRealm(),
		},
	})
}

// handleStatus returns system status
func (m *Manager) handleStatus(c *gin.Context) {
	// Get CPU usage
	cpuPercent, _ := cpu.Percent(time.Second, false)

	// Get memory info
	vmem, _ := mem.VirtualMemory()

	c.JSON(http.StatusOK, gin.H{
		"status": "online",
		"system": gin.H{
			"cpu_percent":    cpuPercent,
			"memory_percent": vmem.UsedPercent,
			"memory_total":   vmem.Total,
			"memory_used":    vmem.Used,
		},
		"uptime": time.Now().Unix(),
	})
}

// handleBoard returns board configuration
func (m *Manager) handleBoard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"uuid":       m.board.UUID,
		"code":       m.board.Code,
		"name":       m.board.Name,
		"type":       m.board.Type,
		"status":     m.board.Status,
		"mobile":     m.board.Mobile,
		"agent":      m.board.Agent,
		"created_at": m.board.CreatedAt,
		"updated_at": m.board.UpdatedAt,
		"location":   m.board.Location,
		"extra":      m.board.Extra,
	})
}

// handleHome renders the home page
func (m *Manager) handleHome(c *gin.Context) {
	tmpl, err := template.ParseFS(templates, "templates/home.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	hostname, _ := os.Hostname()

	data := gin.H{
		"Title":    "Lightning-rod Dashboard",
		"Board":    m.board.Name,
		"UUID":     m.board.UUID,
		"Type":     m.board.Type,
		"Status":   m.board.Status,
		"Hostname": hostname,
	}

	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// handleDashboard renders the dashboard page
func (m *Manager) handleDashboard(c *gin.Context) {
	c.Redirect(http.StatusFound, "/")
}
