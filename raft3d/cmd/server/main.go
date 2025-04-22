package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JyiaJain/raft3d/internal/api"
	"github.com/JyiaJain/raft3d/internal/config"
	"github.com/JyiaJain/raft3d/internal/raft"
	"github.com/JyiaJain/raft3d/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Raft node
	joinExisting := cfg.JoinAddr != ""
	raftNode, err := raft.NewRaftNode(cfg.NodeID, cfg.RaftBindAddr, cfg.AdvertisedAddr, cfg.DataDir, joinExisting, cfg.Bootstrap)
	if err != nil {
		log.Fatalf("Failed to create Raft node: %v", err)
	}

	// Create service
	svc := service.NewService(raftNode)

	// If joining an existing cluster, send join request
	if joinExisting {
		// Wait a bit for the Raft node to initialize
		time.Sleep(3 * time.Second)
		
		// Try to join the cluster
		if err := joinCluster(cfg.JoinAddr, cfg.NodeID, cfg.AdvertisedAddr); err != nil {
			log.Printf("Failed to join cluster: %v", err)
			// Continue anyway, we'll try to operate as a standalone node
		}
	}

	// Create API router
	router := api.SetupRouter(svc)

	// Start HTTP server
	httpServer := &http.Server{
		Addr:    cfg.APIBindAddr,
		Handler: router,
	}
	
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	
	log.Printf("Raft3D node %s started. HTTP API available at http://%s", cfg.NodeID, cfg.APIBindAddr)
	
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Println("Shutting down server...")
	
	// Shutdown HTTP server
	httpServer.Shutdown(nil)
	
	// Shutdown Raft node
	if err := raftNode.Shutdown(); err != nil {
		log.Printf("Error shutting down Raft node: %v", err)
	}
	
	log.Println("Server gracefully stopped")
}

// joinCluster sends a join request to an existing node
func joinCluster(leaderAddr, nodeID, raftAddr string) error {
	// Prepare join request
	data := map[string]string{
		"node_id": nodeID,
		"addr":    raftAddr,
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	// Send request
	url := fmt.Sprintf("http://%s/api/v1/cluster/join", leaderAddr)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}