package service

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/JyiaJain/raft3d/internal/models"
	"github.com/JyiaJain/raft3d/internal/raft"
)

// Service provides high-level operations for the Raft3D application
type Service struct {
	raftNode *raft.RaftNode
}

// NewService creates a new Service
func NewService(raftNode *raft.RaftNode) *Service {
	return &Service{
		raftNode: raftNode,
	}
}

// CreatePrinter creates a new printer
func (s *Service) CreatePrinter(printer *models.Printer) (*models.Printer, error) {
	// Generate ID if not provided
	if printer.ID == "" {
		printer.ID = uuid.New().String()
	}

	if err := s.raftNode.CreatePrinter(printer); err != nil {
		return nil, fmt.Errorf("failed to create printer: %w", err)
	}

	return printer, nil
}

// GetPrinters returns all printers
func (s *Service) GetPrinters() []*models.Printer {
	return s.raftNode.GetPrinters()
}

// CreateFilament creates a new filament
func (s *Service) CreateFilament(filament *models.Filament) (*models.Filament, error) {
	// Generate ID if not provided
	if filament.ID == "" {
		filament.ID = uuid.New().String()
	}

	if err := s.raftNode.CreateFilament(filament); err != nil {
		return nil, fmt.Errorf("failed to create filament: %w", err)
	}

	return filament, nil
}

// GetFilaments returns all filaments
func (s *Service) GetFilaments() []*models.Filament {
	return s.raftNode.GetFilaments()
}

// CreatePrintJob creates a new print job
func (s *Service) CreatePrintJob(job *models.PrintJob) (*models.PrintJob, error) {
	// Generate ID if not provided
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	// Validate printer exists
	if _, exists := s.raftNode.GetPrinter(job.PrinterID); !exists {
		return nil, fmt.Errorf("printer with ID %s does not exist", job.PrinterID)
	}

	// Validate filament exists
	filament, exists := s.raftNode.GetFilament(job.FilamentID)
	if !exists {
		return nil, fmt.Errorf("filament with ID %s does not exist", job.FilamentID)
	}

	// Calculate remaining filament weight considering existing jobs
	remainingWeight := filament.RemainingWeightInGrams
	for _, existingJob := range s.raftNode.GetPrintJobs() {
		if existingJob.FilamentID == job.FilamentID && 
		   (existingJob.Status == "Queued" || existingJob.Status == "Running") {
			remainingWeight -= existingJob.PrintWeightInGrams
		}
	}

	// Check if there's enough filament for this job
	if job.PrintWeightInGrams > remainingWeight {
		return nil, fmt.Errorf("not enough filament remaining (need %d, have %d)", 
							  job.PrintWeightInGrams, remainingWeight)
	}

	// Set initial status
	job.Status = "Queued"

	if err := s.raftNode.CreatePrintJob(job); err != nil {
		return nil, fmt.Errorf("failed to create print job: %w", err)
	}

	return job, nil
}

// GetPrintJobs returns all print jobs
func (s *Service) GetPrintJobs() []*models.PrintJob {
	return s.raftNode.GetPrintJobs()
}

// UpdatePrintJobStatus updates the status of a print job
func (s *Service) UpdatePrintJobStatus(jobID, status string) (*models.PrintJob, error) {
	// Get the job
	job, exists := s.raftNode.GetPrintJob(jobID)
	if !exists {
		return nil, fmt.Errorf("print job with ID %s does not exist", jobID)
	}

	// Validate status transition
	if err := models.ValidateStatusTransition(job.Status, status); err != nil {
		return nil, err
	}

	// Update status
	if err := s.raftNode.UpdatePrintJobStatus(jobID, status); err != nil {
		return nil, fmt.Errorf("failed to update print job status: %w", err)
	}

	// Get updated job
	updatedJob, _ := s.raftNode.GetPrintJob(jobID)
	return updatedJob, nil
}

// IsLeader returns true if this node is the leader
func (s *Service) IsLeader() bool {
	return s.raftNode.IsLeader()
}

// GetLeaderAddress returns the address of the current leader
func (s *Service) GetLeaderAddress() string {
	return s.raftNode.LeaderAddress()
}

// JoinCluster joins an existing Raft cluster
func (s *Service) JoinCluster(nodeID, addr string) error {
	if !s.IsLeader() {
		return errors.New("not the leader")
	}
	return s.raftNode.Join(nodeID, addr)
}

// LeaveCluster removes a node from the Raft cluster
func (s *Service) LeaveCluster(nodeID string) error {
	if !s.IsLeader() {
		return errors.New("not the leader")
	}
	return s.raftNode.Leave(nodeID)
}