package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// 应用配置
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Weather WeatherConfig `yaml:"weather"`
	Media   MediaConfig   `yaml:"media"`
}

// HTTP Server 配置
type ServerConfig struct {
	Address string `yaml:"address"`
}

// 天气配置
type WeatherConfig struct {
	APIHost      string   `yaml:"api_host"`
	DeveloperID  string   `yaml:"developer_id"`
	ProjectID    string   `yaml:"project_id"`
	CredentialID string   `yaml:"credential_id"`
	PrivateKey   string   `yaml:"private_key"`
	Longitude    *float64 `yaml:"longitude"`
	Latitude     *float64 `yaml:"latitude"`
}

// 媒体配置
type MediaConfig struct {
	Steam SteamConfig `yaml:"steam"`
	Music MusicConfig `yaml:"music"`
}

// Steam 配置
type SteamConfig struct {
	APIKey  string `yaml:"api_key"`
	SteamID string `yaml:"steam_id"`
}

// 音乐配置
type MusicConfig struct {
	NetEase NetEaseConfig `yaml:"netease"`
}

// 网易云音乐配置
type NetEaseConfig struct {
	UserID int64 `yaml:"user_id"`
}

// 加载配置
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}
	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// 校验配置
func validate(cfg Config) error {
	if strings.TrimSpace(cfg.Server.Address) == "" {
		return errors.New("server.address must not be empty")
	}
	if strings.TrimSpace(cfg.Weather.APIHost) == "" {
		return errors.New("weather.api_host is required")
	}
	if strings.ContainsAny(cfg.Weather.APIHost, "/?#@") || strings.Contains(cfg.Weather.APIHost, "://") {
		return errors.New("weather.api_host must be a host name")
	}
	if strings.TrimSpace(cfg.Weather.DeveloperID) == "" {
		return errors.New("weather.developer_id is required")
	}
	if strings.TrimSpace(cfg.Weather.ProjectID) == "" {
		return errors.New("weather.project_id is required")
	}
	if strings.TrimSpace(cfg.Weather.CredentialID) == "" {
		return errors.New("weather.credential_id is required")
	}
	if strings.TrimSpace(cfg.Weather.PrivateKey) == "" {
		return errors.New("weather.private_key is required")
	}
	if err := validatePrivateKeyPath(cfg.Weather.PrivateKey); err != nil {
		return err
	}
	if cfg.Weather.Longitude == nil {
		return errors.New("weather.longitude must be set")
	}
	if math.IsNaN(*cfg.Weather.Longitude) || math.IsInf(*cfg.Weather.Longitude, 0) || *cfg.Weather.Longitude < -180 || *cfg.Weather.Longitude > 180 {
		return errors.New("weather.longitude must be between -180 and 180")
	}
	if cfg.Weather.Latitude == nil {
		return errors.New("weather.latitude must be set")
	}
	if math.IsNaN(*cfg.Weather.Latitude) || math.IsInf(*cfg.Weather.Latitude, 0) || *cfg.Weather.Latitude < -90 || *cfg.Weather.Latitude > 90 {
		return errors.New("weather.latitude must be between -90 and 90")
	}
	if strings.TrimSpace(cfg.Media.Steam.APIKey) == "" {
		return errors.New("media.steam.api_key must not be empty")
	}
	if strings.TrimSpace(cfg.Media.Steam.SteamID) == "" {
		return errors.New("media.steam.steam_id must not be empty")
	}
	if cfg.Media.Music.NetEase.UserID <= 0 {
		return errors.New("media.music.netease.user_id must be greater than zero")
	}

	return nil
}

// 校验私钥路径
func validatePrivateKeyPath(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("weather.private_key must reference a readable regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return errors.New("weather.private_key must reference a readable regular file")
	}
	if err := file.Close(); err != nil {
		return errors.New("failed to close weather.private_key")
	}

	return nil
}
