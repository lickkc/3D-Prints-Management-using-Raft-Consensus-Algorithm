package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Command represents an operation to be applied to the FSM
type Command struct {
	Op    string          `json:"op"`
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// Printer represents a 3D printer
type Printer struct {
	ID      string `json:"id"`
	Company string `json:"company"`
	Model   string `json:"model"`
}

// Filament represents a roll of plastic filament
type Filament struct {
	ID                    string `json:"id"`
	Type                  string `json:"type"`
	Color                 string `json:"color"`
	TotalWeightInGrams    int    `json:"total_weight_in_grams"`
	RemainingWeightInGrams int   `json:"remaining_weight_in_grams"`
}

// PrintJob represents a job to print an item
type PrintJob struct {
	ID               string `json:"id"`
	PrinterID        string `json:"printer_id"`
	FilamentID       string `json:"filament_id"`
	Filepath         string `json:"filepath"`
	PrintWeightInGrams int  `json:"print_weight_in_grams"`
	Status           string `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

// Valid filament types
var ValidFilamentTypes = map[string]bool{
	"PLA":  true,
	"PETG": true,
	"ABS":  true,
	"TPU":  true,
}

// Valid print job statuses
var ValidPrintJobStatuses = map[string]bool{
	"Queued":   true,
	"Running":  true,
	"Done":     true,
	"Cancelled": true,
}

// ValidateFilament ensures the filament data is valid
func (f *Filament) Validate() error {
	if f.ID == "" {
		return errors.New("filament ID cannot be empty")
	}
	
	if !ValidFilamentTypes[f.Type] {
		return fmt.Errorf("invalid filament type: %s", f.Type)
	}
	
	if f.Color == "" {
		return errors.New("filament color cannot be empty")
	}
	
	if f.TotalWeightInGrams <= 0 {
		return errors.New("total weight must be positive")
	}
	
	if f.RemainingWeightInGrams < 0 || f.RemainingWeightInGrams > f.TotalWeightInGrams {
		return errors.New("remaining weight must be between 0 and total weight")
	}
	
	return nil
}

// ValidatePrinter ensures the printer data is valid
func (p *Printer) Validate() error {
	if p.ID == "" {
		return errors.New("printer ID cannot be empty")
	}
	
	if p.Company == "" {
		return errors.New("printer company cannot be empty")
	}
	
	if p.Model == "" {
		return errors.New("printer model cannot be empty")
	}
	
	return nil
}

// ValidatePrintJob ensures the print job data is valid
func (pj *PrintJob) Validate() error {
	if pj.ID == "" {
		return errors.New("print job ID cannot be empty")
	}
	
	if pj.PrinterID == "" {
		return errors.New("printer ID cannot be empty")
	}
	
	if pj.FilamentID == "" {
		return errors.New("filament ID cannot be empty")
	}
	
	if pj.Filepath == "" {
		return errors.New("filepath cannot be empty")
	}
	
	if pj.PrintWeightInGrams <= 0 {
		return errors.New("print weight must be positive")
	}
	
	return nil
}

// ValidateStatusTransition checks if a status transition is valid
func ValidateStatusTransition(currentStatus, newStatus string) error {
	if !ValidPrintJobStatuses[newStatus] {
		return fmt.Errorf("invalid status: %s", newStatus)
	}
	
	switch currentStatus {
	case "Queued":
		if newStatus != "Running" && newStatus != "Cancelled" {
			return errors.New("job in Queued state can only transition to Running or Cancelled")
		}
	case "Running":
		if newStatus != "Done" && newStatus != "Cancelled" {
			return errors.New("job in Running state can only transition to Done or Cancelled")
		}
	case "Done", "Cancelled":
		return errors.New("job in terminal state cannot change status")
	}
	
	return nil
}