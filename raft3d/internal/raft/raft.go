package raft

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/JyiaJain/raft3d/internal/models"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

// RaftNode is a wrapper around the Raft implementation
type RaftNode struct {
	raft *raft.Raft
	fsm  *FSM
}

// NewRaftNode creates a new RaftNode
func NewRaftNode(nodeID string, bindAddr string, advertiseAddr string, dataDir string, joinExisting bool, bootstrap bool) (*RaftNode, error) {
	// Create the FSM
	fsm, err := NewFSM(filepath.Join(dataDir, "raft3d.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	// Create Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)
	config.SnapshotInterval = 1 * time.Minute
	config.SnapshotThreshold = 1024
	config.TrailingLogs = 10000

	// Setup Raft communication


	// Use the advertised address for the transport if provided
	advertiseAddrObj, err := net.ResolveTCPAddr("tcp", advertiseAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve advertised TCP address: %w", err)
	}

	transport, err := raft.NewTCPTransport(bindAddr, advertiseAddrObj, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP transport: %w", err)
	}

	// Create the snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(dataDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft-log.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft-stable.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create stable store: %w", err)
	}

	// Instantiate the Raft system
	r, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %w", err)
	}

	if !joinExisting && bootstrap {
		// Bootstrap the cluster if this is the first node
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		r.BootstrapCluster(configuration)
	}

	return &RaftNode{
		raft: r,
		fsm:  fsm,
	}, nil
}

// Join adds a new node to the Raft cluster
func (r *RaftNode) Join(nodeID, addr string) error {
	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return fmt.Errorf("failed to get raft configuration: %w", err)
	}

	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				return nil
			}

			future := r.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s: %w", nodeID, err)
			}
		}
	}

	future := r.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("error joining node %s at %s: %w", nodeID, addr, err)
	}

	return nil
}

// Leave removes the node from the Raft cluster
func (r *RaftNode) Leave(nodeID string) error {
	future := r.raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("error leaving the cluster: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the Raft node
func (r *RaftNode) Shutdown() error {
	return r.raft.Shutdown().Error()
}

// IsLeader returns true if this node is the leader
func (r *RaftNode) IsLeader() bool {
	return r.raft.State() == raft.Leader
}

// LeaderAddress returns the address of the current leader
func (r *RaftNode) LeaderAddress() string {
	return string(r.raft.Leader())
}

// GetState returns the current FSM state
func (r *RaftNode) GetState() (*FSM, error) {
	return r.fsm, nil
}

// CreatePrinter creates a new printer
func (r *RaftNode) CreatePrinter(printer *models.Printer) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	data, err := json.Marshal(printer)
	if err != nil {
		return err
	}

	cmd := models.Command{
		Op:    "create_printer",
		Key:   printer.ID,
		Value: data,
	}

	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	future := r.raft.Apply(cmdBytes, raftTimeout)
	if err := future.Error(); err != nil {
		return err
	}

	res := future.Response()
	if err, ok := res.(error); ok {
		return err
	}

	return nil
}

// CreateFilament creates a new filament
func (r *RaftNode) CreateFilament(filament *models.Filament) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	data, err := json.Marshal(filament)
	if err != nil {
		return err
	}

	cmd := models.Command{
		Op:    "create_filament",
		Key:   filament.ID,
		Value: data,
	}

	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	future := r.raft.Apply(cmdBytes, raftTimeout)
	if err := future.Error(); err != nil {
		return err
	}

	res := future.Response()
	if err, ok := res.(error); ok {
		return err
	}

	return nil
}

// CreatePrintJob creates a new print job
func (r *RaftNode) CreatePrintJob(job *models.PrintJob) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	cmd := models.Command{
		Op:    "create_printjob",
		Key:   job.ID,
		Value: data,
	}

	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	future := r.raft.Apply(cmdBytes, raftTimeout)
	if err := future.Error(); err != nil {
		return err
	}

	res := future.Response()
	if err, ok := res.(error); ok {
		return err
	}

	return nil
}

// UpdatePrintJobStatus updates the status of a print job
func (r *RaftNode) UpdatePrintJobStatus(jobID, status string) error {
	if !r.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	data, err := json.Marshal(struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		ID:     jobID,
		Status: status,
	})
	if err != nil {
		return err
	}

	cmd := models.Command{
		Op:    "update_printjob_status",
		Key:   jobID,
		Value: data,
	}

	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	future := r.raft.Apply(cmdBytes, raftTimeout)
	if err := future.Error(); err != nil {
		return err
	}

	res := future.Response()
	if err, ok := res.(error); ok {
		return err
	}

	return nil
}

// GetPrinters returns all printers
func (r *RaftNode) GetPrinters() []*models.Printer {
	return r.fsm.GetPrinters()
}

// GetFilaments returns all filaments
func (r *RaftNode) GetFilaments() []*models.Filament {
	return r.fsm.GetFilaments()
}

// GetPrintJobs returns all print jobs
func (r *RaftNode) GetPrintJobs() []*models.PrintJob {
	return r.fsm.GetPrintJobs()
}

// GetPrinter returns a printer by ID
func (r *RaftNode) GetPrinter(id string) (*models.Printer, bool) {
	return r.fsm.GetPrinter(id)
}

// GetFilament returns a filament by ID
func (r *RaftNode) GetFilament(id string) (*models.Filament, bool) {
	return r.fsm.GetFilament(id)
}

// GetPrintJob returns a print job by ID
func (r *RaftNode) GetPrintJob(id string) (*models.PrintJob, bool) {
	return r.fsm.GetPrintJob(id)
}
