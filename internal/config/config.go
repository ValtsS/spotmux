package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ServiceCallsign          string   `json:"service_callsign"`
	OperatorName             string   `json:"operator_name"`
	OperatorQTH              string   `json:"operator_qth"`
	OperatorLocation         string   `json:"operator_location"`
	ListenPort               int      `json:"listen_port"`
	MaxDownstreamConnections int      `json:"max_downstream_connections"`
	MaxSpotsPerMinute        int      `json:"max_spots_per_minute"`
	SpotCacheSize            int      `json:"spot_cache_size"`
	DuplicateWindowSeconds   int      `json:"duplicate_window_seconds"`
	UpstreamClusters         []string `json:"upstream_clusters"`
	IgnoredPrefixes          []string `json:"ignored_prefixes"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Set defaults if needed
	if cfg.SpotCacheSize == 0 {
		cfg.SpotCacheSize = 200
	}
	if cfg.DuplicateWindowSeconds == 0 {
		cfg.DuplicateWindowSeconds = 5
	}
	if cfg.MaxSpotsPerMinute == 0 {
		cfg.MaxSpotsPerMinute = 5
	}
	if cfg.MaxDownstreamConnections == 0 {
		cfg.MaxDownstreamConnections = 10
	}
	if len(cfg.IgnoredPrefixes) == 0 {
		cfg.IgnoredPrefixes = []string{
			"R", "UA", "UB", "UC", "UD", "UE", "UF", "UG", "UH", "UI",
			"EU", "EW",
			"UA2", "RA2",
		}
	}

	return &cfg, nil
}
