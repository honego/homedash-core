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
}

// HTTP Server 配置
type ServerConfig struct {
	Address string `yaml:"address"`
}

// 天气配置
type WeatherConfig struct {
	QWeatherAPIHost      string   `yaml:"qweather_api_host"`
	QWeatherDeveloperID  string   `yaml:"qweather_developer_id"`
	QWeatherProjectID    string   `yaml:"qweather_project_id"`
	QWeatherCredentialID string   `yaml:"qweather_credential_id"`
	QWeatherPrivateKey   string   `yaml:"qweather_private_key"`
	Longitude            *float64 `yaml:"longitude"`
	Latitude             *float64 `yaml:"latitude"`
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
	if strings.TrimSpace(cfg.Weather.QWeatherAPIHost) == "" {
		return errors.New("weather.qweather_api_host must not be empty")
	}
	if strings.ContainsAny(cfg.Weather.QWeatherAPIHost, "/?#@") || strings.Contains(cfg.Weather.QWeatherAPIHost, "://") {
		return errors.New("weather.qweather_api_host must be a host name")
	}
	if strings.TrimSpace(cfg.Weather.QWeatherDeveloperID) == "" {
		return errors.New("weather.qweather_developer_id must not be empty")
	}
	if strings.TrimSpace(cfg.Weather.QWeatherProjectID) == "" {
		return errors.New("weather.qweather_project_id must not be empty")
	}
	if strings.TrimSpace(cfg.Weather.QWeatherCredentialID) == "" {
		return errors.New("weather.qweather_credential_id must not be empty")
	}
	if strings.TrimSpace(cfg.Weather.QWeatherPrivateKey) == "" {
		return errors.New("weather.qweather_private_key must not be empty")
	}
	if err := validatePrivateKeyPath(cfg.Weather.QWeatherPrivateKey); err != nil {
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

	return nil
}

// 校验私钥路径
func validatePrivateKeyPath(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("weather.qweather_private_key must reference a readable regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return errors.New("weather.qweather_private_key must reference a readable regular file")
	}
	if err := file.Close(); err != nil {
		return errors.New("failed to close weather.qweather_private_key")
	}

	return nil
}
