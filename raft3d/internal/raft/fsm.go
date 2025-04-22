package raft

import (
	"encoding/json"
	// "errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/JyiaJain/raft3d/internal/models"
	bolt "go.etcd.io/bbolt"
)

// Store buckets
var (
	printerBucket  = []byte("printers")
	filamentBucket = []byte("filaments")
	printJobBucket = []byte("printjobs")
)

// FSM implements the raft.FSM interface for the Raft3D application
type FSM struct {
	mu       sync.RWMutex
	db       *bolt.DB
	printers map[string]*models.Printer
	filaments map[string]*models.Filament
	printJobs map[string]*models.PrintJob
}

// NewFSM creates a new FSM with the given db path
func NewFSM(dbPath string) (*FSM, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{printerBucket, filamentBucket, printJobBucket} {
			_, err := tx.CreateBucketIfNotExists(bucket)
			if err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	fsm := &FSM{
		db:       db,
		printers: make(map[string]*models.Printer),
		filaments: make(map[string]*models.Filament),
		printJobs: make(map[string]*models.PrintJob),
	}

	// Load data from DB
	if err := fsm.loadFromDB(); err != nil {
		return nil, fmt.Errorf("failed to load data from db: %w", err)
	}

	return fsm, nil
}

// Apply logs is called once a log entry is committed
// Returns a value that will be made available in the ApplyFuture returned by Raft.Apply
func (f *FSM) Apply(log *raft.Log) interface{} {
	var cmd models.Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var result interface{}
	var err error

	switch cmd.Op {
	case "create_printer":
		var printer models.Printer
		if err := json.Unmarshal(cmd.Value, &printer); err != nil {
			return fmt.Errorf("failed to unmarshal printer: %w", err)
		}
		
		err = f.createPrinter(&printer)
		result = &printer
		
	case "create_filament":
		var filament models.Filament
		if err := json.Unmarshal(cmd.Value, &filament); err != nil {
			return fmt.Errorf("failed to unmarshal filament: %w", err)
		}
		
		err = f.createFilament(&filament)
		result = &filament

	case "create_printjob":
		var printJob models.PrintJob
		if err := json.Unmarshal(cmd.Value, &printJob); err != nil {
			return fmt.Errorf("failed to unmarshal print job: %w", err)
		}
		
		printJob.Status = "Queued"
		printJob.CreatedAt = time.Now()
		err = f.createPrintJob(&printJob)
		result = &printJob

	case "update_printjob_status":
		var data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		
		if err := json.Unmarshal(cmd.Value, &data); err != nil {
			return fmt.Errorf("failed to unmarshal status update data: %w", err)
		}
		
		err = f.updatePrintJobStatus(data.ID, data.Status)
		if err == nil {
			result = f.printJobs[data.ID]
		}

	default:
		err = fmt.Errorf("unknown command: %s", cmd.Op)
	}

	if err != nil {
		return err
	}
	
	return result
}

// Snapshot returns an FSM snapshot
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a copy of the state
	printers := make(map[string]*models.Printer)
	filaments := make(map[string]*models.Filament)
	printJobs := make(map[string]*models.PrintJob)

	for k, v := range f.printers {
		printers[k] = v
	}
	
	for k, v := range f.filaments {
		filaments[k] = v
	}
	
	for k, v := range f.printJobs {
		printJobs[k] = v
	}

	return &FSMSnapshot{
		printers:  printers,
		filaments: filaments,
		printJobs: printJobs,
	}, nil
}

// Restore restores an FSM from a snapshot
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	var snap FSMSnapshot
	if err := json.NewDecoder(rc).Decode(&snap); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.printers = snap.printers
	f.filaments = snap.filaments
	f.printJobs = snap.printJobs

	// Persist to DB
	return f.saveToDB()
}

// GetPrinters returns all printers
func (f *FSM) GetPrinters() []*models.Printer {
	f.mu.RLock()
	defer f.mu.RUnlock()

	printers := make([]*models.Printer, 0, len(f.printers))
	for _, printer := range f.printers {
		printers = append(printers, printer)
	}
	
	return printers
}

// GetFilaments returns all filaments
func (f *FSM) GetFilaments() []*models.Filament {
	f.mu.RLock()
	defer f.mu.RUnlock()

	filaments := make([]*models.Filament, 0, len(f.filaments))
	for _, filament := range f.filaments {
		filaments = append(filaments, filament)
	}
	
	return filaments
}

// GetPrintJobs returns all print jobs
func (f *FSM) GetPrintJobs() []*models.PrintJob {
	f.mu.RLock()
	defer f.mu.RUnlock()

	printJobs := make([]*models.PrintJob, 0, len(f.printJobs))
	for _, job := range f.printJobs {
		printJobs = append(printJobs, job)
	}
	
	return printJobs
}

// GetPrinter returns a printer by ID
func (f *FSM) GetPrinter(id string) (*models.Printer, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	printer, exists := f.printers[id]
	return printer, exists
}

// GetFilament returns a filament by ID
func (f *FSM) GetFilament(id string) (*models.Filament, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	filament, exists := f.filaments[id]
	return filament, exists
}

// GetPrintJob returns a print job by ID
func (f *FSM) GetPrintJob(id string) (*models.PrintJob, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	job, exists := f.printJobs[id]
	return job, exists
}

// Internal methods for state manipulation

func (f *FSM) createPrinter(printer *models.Printer) error {
	if err := printer.Validate(); err != nil {
		return err
	}

	if _, exists := f.printers[printer.ID]; exists {
		return fmt.Errorf("printer with ID %s already exists", printer.ID)
	}

	f.printers[printer.ID] = printer
	return f.savePrinterToDB(printer)
}

func (f *FSM) createFilament(filament *models.Filament) error {
	if err := filament.Validate(); err != nil {
		return err
	}

	if _, exists := f.filaments[filament.ID]; exists {
		return fmt.Errorf("filament with ID %s already exists", filament.ID)
	}

	f.filaments[filament.ID] = filament
	return f.saveFilamentToDB(filament)
}

func (f *FSM) createPrintJob(job *models.PrintJob) error {
	if err := job.Validate(); err != nil {
		return err
	}

	// Check if printer exists
	if _, exists := f.printers[job.PrinterID]; !exists {
		return fmt.Errorf("printer with ID %s does not exist", job.PrinterID)
	}

	// Check if filament exists
	filament, exists := f.filaments[job.FilamentID]
	if !exists {
		return fmt.Errorf("filament with ID %s does not exist", job.FilamentID)
	}

	// Calculate remaining filament weight considering existing jobs
	remainingWeight := filament.RemainingWeightInGrams
	for _, existingJob := range f.printJobs {
		if existingJob.FilamentID == job.FilamentID && 
		   (existingJob.Status == "Queued" || existingJob.Status == "Running") {
			remainingWeight -= existingJob.PrintWeightInGrams
		}
	}

	// Check if there's enough filament for this job
	if job.PrintWeightInGrams > remainingWeight {
		return fmt.Errorf("not enough filament remaining (need %d, have %d)", 
						  job.PrintWeightInGrams, remainingWeight)
	}

	f.printJobs[job.ID] = job
	return f.savePrintJobToDB(job)
}

func (f *FSM) updatePrintJobStatus(jobID, newStatus string) error {
	job, exists := f.printJobs[jobID]
	if !exists {
		return fmt.Errorf("print job with ID %s does not exist", jobID)
	}

	// Validate status transition
	if err := models.ValidateStatusTransition(job.Status, newStatus); err != nil {
		return err
	}

	oldStatus := job.Status
	job.Status = newStatus

	// If job is done, reduce filament weight
	if newStatus == "Done" && oldStatus != "Done" {
		filament, exists := f.filaments[job.FilamentID]
		if !exists {
			return fmt.Errorf("filament with ID %s does not exist", job.FilamentID)
		}

		filament.RemainingWeightInGrams -= job.PrintWeightInGrams
		if filament.RemainingWeightInGrams < 0 {
			filament.RemainingWeightInGrams = 0
		}

		// Save updated filament
		if err := f.saveFilamentToDB(filament); err != nil {
			return err
		}
	}

	return f.savePrintJobToDB(job)
}

// DB operations

func (f *FSM) loadFromDB() error {
	// Load printers
	err := f.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(printerBucket)
		
		return b.ForEach(func(k, v []byte) error {
			var printer models.Printer
			if err := json.Unmarshal(v, &printer); err != nil {
				return err
			}
			f.printers[printer.ID] = &printer
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to load printers: %w", err)
	}

	// Load filaments
	err = f.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(filamentBucket)
		
		return b.ForEach(func(k, v []byte) error {
			var filament models.Filament
			if err := json.Unmarshal(v, &filament); err != nil {
				return err
			}
			f.filaments[filament.ID] = &filament
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to load filaments: %w", err)
	}

	// Load print jobs
	err = f.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(printJobBucket)
		
		return b.ForEach(func(k, v []byte) error {
			var job models.PrintJob
			if err := json.Unmarshal(v, &job); err != nil {
				return err
			}
			f.printJobs[job.ID] = &job
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to load print jobs: %w", err)
	}

	return nil
}

func (f *FSM) saveToDB() error {
	// Save printers
	for _, printer := range f.printers {
		if err := f.savePrinterToDB(printer); err != nil {
			return err
		}
	}

	// Save filaments
	for _, filament := range f.filaments {
		if err := f.saveFilamentToDB(filament); err != nil {
			return err
		}
	}

	// Save print jobs
	for _, job := range f.printJobs {
		if err := f.savePrintJobToDB(job); err != nil {
			return err
		}
	}

	return nil
}

func (f *FSM) savePrinterToDB(printer *models.Printer) error {
	return f.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(printerBucket)
		
		data, err := json.Marshal(printer)
		if err != nil {
			return err
		}
		
		return b.Put([]byte(printer.ID), data)
	})
}

func (f *FSM) saveFilamentToDB(filament *models.Filament) error {
	return f.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(filamentBucket)
		
		data, err := json.Marshal(filament)
		if err != nil {
			return err
		}
		
		return b.Put([]byte(filament.ID), data)
	})
}

func (f *FSM) savePrintJobToDB(job *models.PrintJob) error {
	return f.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(printJobBucket)
		
		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		
		return b.Put([]byte(job.ID), data)
	})
}

// FSMSnapshot is used for taking snapshots of the FSM state
type FSMSnapshot struct {
	printers  map[string]*models.Printer
	filaments map[string]*models.Filament
	printJobs map[string]*models.PrintJob
}

// Persist writes the snapshot to the given sink
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s)
	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if _, err := sink.Write(data); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return sink.Close()
}

// Release is invoked when we are finished with the snapshot
func (s *FSMSnapshot) Release() {}