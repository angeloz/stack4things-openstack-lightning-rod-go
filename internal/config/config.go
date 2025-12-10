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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultSettingsFile = "/etc/iotronic/settings.json"
)

// Config represents the Lightning Rod configuration
type Config struct {
	LightningRod LightningRodConfig `mapstructure:"lightningrod"`
	Autobahn     AutobahnConfig     `mapstructure:"autobahn"`
	Services     ServicesConfig     `mapstructure:"services"`
	WebServices  WebServicesConfig  `mapstructure:"webservices"`
}

// LightningRodConfig contains core Lightning Rod settings
type LightningRodConfig struct {
	Home           string `mapstructure:"home"`
	LogLevel       string `mapstructure:"log_level"`
	LogFile        string `mapstructure:"log_file"`
	SkipCertVerify bool   `mapstructure:"skip_cert_verify"`
}

// AutobahnConfig contains WAMP/Autobahn settings
type AutobahnConfig struct {
	ConnectionTimer        int `mapstructure:"connection_timer"`
	AliveTimer             int `mapstructure:"alive_timer"`
	RPCAliveTimer          int `mapstructure:"rpc_alive_timer"`
	ConnectionFailureTimer int `mapstructure:"connection_failure_timer"`
}

// ServicesConfig contains service manager settings
type ServicesConfig struct {
	WstunBin string `mapstructure:"wstun_bin"`
}

// WebServicesConfig contains webservice manager settings
type WebServicesConfig struct {
	Proxy string `mapstructure:"proxy"`
}

// BoardSettings represents the board configuration from settings.json
type BoardSettings struct {
	Iotronic IotronicSettings `json:"iotronic"`
}

// IotronicSettings contains IoTronic-specific board settings
type IotronicSettings struct {
	Board BoardConfig       `json:"board"`
	WAMP  WampConfiguration `json:"wamp"`
	Extra map[string]any    `json:"extra"`
}

// BoardConfig contains board identification and status
type BoardConfig struct {
	UUID      string         `json:"uuid"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Type      string         `json:"type"`
	Mobile    bool           `json:"mobile"`
	Agent     string         `json:"agent"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	Location  map[string]any `json:"location"`
	Extra     map[string]any `json:"extra"`
}

// WampConfiguration contains WAMP connection settings
type WampConfiguration struct {
	MainAgent         *WampAgent `json:"main-agent,omitempty"`
	RegistrationAgent *WampAgent `json:"registration-agent,omitempty"`
}

// WampAgent represents a WAMP agent connection
type WampAgent struct {
	URL   string `json:"url"`
	Realm string `json:"realm"`
}

// Load loads configuration from file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file path
	v.SetConfigFile(configPath)
	v.SetConfigType("ini")

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		// If config file doesn't exist, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// LoadBoardSettings loads board settings from settings.json
func LoadBoardSettings(home string) (*BoardSettings, error) {
	settingsPath := filepath.Join(home, "settings.json")
	if home == "" {
		settingsPath = DefaultSettingsFile
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings BoardSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings file: %w", err)
	}

	return &settings, nil
}

// SaveBoardSettings saves board settings to settings.json
func SaveBoardSettings(home string, settings *BoardSettings) error {
	settingsPath := filepath.Join(home, "settings.json")
	if home == "" {
		settingsPath = DefaultSettingsFile
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func setDefaults(v *viper.Viper) {
	// Lightning Rod defaults
	v.SetDefault("lightningrod.home", "/var/lib/iotronic")
	v.SetDefault("lightningrod.log_level", "info")
	v.SetDefault("lightningrod.log_file", "")
	v.SetDefault("lightningrod.skip_cert_verify", true)

	// Autobahn defaults
	v.SetDefault("autobahn.connection_timer", 10)
	v.SetDefault("autobahn.alive_timer", 600)
	v.SetDefault("autobahn.rpc_alive_timer", 3)
	v.SetDefault("autobahn.connection_failure_timer", 600)

	// Services defaults
	v.SetDefault("services.wstun_bin", "/usr/bin/wstun")

	// WebServices defaults
	v.SetDefault("webservices.proxy", "nginx")
}
