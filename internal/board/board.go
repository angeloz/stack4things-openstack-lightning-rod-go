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

package board

import (
	"fmt"
	"sync"
	"time"

	"github.com/MDSLab/iotronic-lightning-rod/internal/config"
	log "github.com/sirupsen/logrus"
)

// Board represents the Lightning Rod board/device
type Board struct {
	mu sync.RWMutex

	// Board identification
	UUID      string
	Code      string
	Name      string
	Status    string
	Type      string
	Mobile    bool
	Agent     string
	CreatedAt string
	UpdatedAt string

	// Location data
	Location map[string]any

	// Extra metadata
	Extra map[string]any

	// Session info
	SessionID string

	// WAMP configuration
	WampConfig *config.WampAgent

	// Configuration
	cfg      *config.Config
	settings *config.BoardSettings
}

// New creates a new Board instance
func New(cfg *config.Config) (*Board, error) {
	b := &Board{
		cfg:      cfg,
		Location: make(map[string]any),
		Extra:    make(map[string]any),
	}

	if err := b.LoadSettings(); err != nil {
		return nil, fmt.Errorf("failed to load board settings: %w", err)
	}

	return b, nil
}

// LoadSettings loads board settings from settings.json
func (b *Board) LoadSettings() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	settings, err := config.LoadBoardSettings(b.cfg.LightningRod.Home)
	if err != nil {
		return err
	}

	b.settings = settings

	// Load board configuration
	boardCfg := settings.Iotronic.Board
	b.UUID = boardCfg.UUID
	b.Code = boardCfg.Code
	b.Name = boardCfg.Name
	b.Status = boardCfg.Status
	b.Type = boardCfg.Type
	b.Mobile = boardCfg.Mobile
	b.Agent = boardCfg.Agent
	b.CreatedAt = boardCfg.CreatedAt
	b.UpdatedAt = boardCfg.UpdatedAt
	b.Location = boardCfg.Location
	b.Extra = boardCfg.Extra

	log.Info("Board settings:")
	log.Infof(" - code: %s", b.Code)
	log.Infof(" - uuid: %s", b.UUID)

	// Load WAMP configuration
	b.loadWampConfig(settings)

	// Handle first boot
	if b.Code == "<REGISTRATION-TOKEN>" {
		log.Info("FIRST BOOT procedure started")
		b.Status = "first_boot"
	}

	return nil
}

func (b *Board) loadWampConfig(settings *config.BoardSettings) {
	if settings.Iotronic.WAMP.MainAgent != nil {
		b.WampConfig = settings.Iotronic.WAMP.MainAgent
		log.Info("WAMP Agent settings:")
	} else if b.Status == "" || b.Status == "registered" || b.Status == "first_boot" {
		b.WampConfig = settings.Iotronic.WAMP.RegistrationAgent
		log.Info("Registration Agent settings:")
	} else {
		log.Error("WAMP Agent configuration is wrong... please check settings.json")
		b.Status = "first_boot"
		return
	}

	log.Infof(" - agent: %s", b.Agent)
	log.Infof(" - url: %s", b.WampConfig.URL)
	log.Infof(" - realm: %s", b.WampConfig.Realm)
}

// UpdateStatus updates the board status and saves to file
func (b *Board) UpdateStatus(status string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Status = status
	b.settings.Iotronic.Board.Status = status

	return config.SaveBoardSettings(b.cfg.LightningRod.Home, b.settings)
}

// SetUpdateTime updates the board's updated_at timestamp
func (b *Board) SetUpdateTime() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02T15:04:05.000000")
	b.UpdatedAt = timestamp
	b.settings.Iotronic.Board.UpdatedAt = timestamp

	return config.SaveBoardSettings(b.cfg.LightningRod.Home, b.settings)
}

// SetConfig updates the entire board configuration
func (b *Board) SetConfig(newSettings *config.BoardSettings) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := config.SaveBoardSettings(b.cfg.LightningRod.Home, newSettings); err != nil {
		return err
	}

	// Reload settings
	b.mu.Unlock()
	err := b.LoadSettings()
	b.mu.Lock()

	return err
}

// GetWampURL returns the WAMP connection URL
func (b *Board) GetWampURL() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.WampConfig != nil {
		return b.WampConfig.URL
	}
	return ""
}

// GetWampRealm returns the WAMP realm
func (b *Board) GetWampRealm() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.WampConfig != nil {
		return b.WampConfig.Realm
	}
	return ""
}

// IsFirstBoot returns whether this is the first boot
func (b *Board) IsFirstBoot() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.Status == "first_boot"
}
