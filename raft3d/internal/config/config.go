package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Config contains the configuration for the application
type Config struct {
	// Node configuration
	NodeID         string
	RaftBindAddr   string
	APIBindAddr    string
	AdvertisedAddr string
	DataDir        string
	JoinAddr       string
	Bootstrap      bool
}

// NewConfig creates a new configuration from command-line flags
func NewConfig() (*Config, error) {
	cfg := &Config{}

	// Define flags
	flag.StringVar(&cfg.NodeID, "id", "", "Node ID (required)")
	flag.StringVar(&cfg.RaftBindAddr, "raft-addr", "0.0.0.0:7000", "Raft bind address")
	flag.StringVar(&cfg.APIBindAddr, "api-addr", "0.0.0.0:8000", "API bind address")
	flag.StringVar(&cfg.AdvertisedAddr, "advertised-addr", "", "Address to advertise to other nodes (defaults to raft-addr)")
	flag.StringVar(&cfg.DataDir, "data-dir", "", "Data directory path (required)")
	flag.StringVar(&cfg.JoinAddr, "join", "", "Address of a node to join (optional)")
	flag.BoolVar(&cfg.Bootstrap, "bootstrap", false, "Bootstrap the cluster with this node")

	// Parse flags
	flag.Parse()

	// Validate required flags
	if cfg.NodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}

	if cfg.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	// If advertised address is not set, use the RaftBindAddr
	if cfg.AdvertisedAddr == "" {
		cfg.AdvertisedAddr = cfg.RaftBindAddr
	}

	// Create data directory if it doesn't exist
	fullPath, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	// Ensure node-specific directory
	nodeDir := filepath.Join(fullPath, cfg.NodeID)
	err = os.MkdirAll(nodeDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Update data dir to node-specific directory
	cfg.DataDir = nodeDir

	return cfg, nil
}