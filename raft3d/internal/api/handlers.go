package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/JyiaJain/raft3d/internal/models"
	"github.com/JyiaJain/raft3d/internal/service"
)

// API provides HTTP endpoints for the Raft3D application
type API struct {
	service *service.Service
}

// NewAPI creates a new API
func NewAPI(service *service.Service) *API {
	return &API{
		service: service,
	}
}

// redirectToLeader redirects requests to the leader if this node is not the leader
func (a *API) redirectToLeader(c *gin.Context) bool {
	if !a.service.IsLeader() {
		leaderAddr := a.service.GetLeaderAddress()
		if leaderAddr == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No leader available"})
			return true
		}

		// Extract the protocol and path
		proto := "http://"
		if c.Request.TLS != nil {
			proto = "https://"
		}

		// Redirect to the leader
		targetURL := fmt.Sprintf("%s%s%s", proto, leaderAddr, c.Request.URL.Path)
		if c.Request.URL.RawQuery != "" {
			targetURL += "?" + c.Request.URL.RawQuery
		}

		c.Redirect(http.StatusTemporaryRedirect, targetURL)
		return true
	}
	return false
}

// CreatePrinter creates a new printer
func (a *API) CreatePrinter(c *gin.Context) {
	if a.redirectToLeader(c) {
		return
	}

	var printer models.Printer
	if err := c.ShouldBindJSON(&printer); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := a.service.CreatePrinter(&printer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetPrinters returns all printers
func (a *API) GetPrinters(c *gin.Context) {
	printers := a.service.GetPrinters()
	c.JSON(http.StatusOK, printers)
}

// CreateFilament creates a new filament
func (a *API) CreateFilament(c *gin.Context) {
	if a.redirectToLeader(c) {
		return
	}

	var filament models.Filament
	if err := c.ShouldBindJSON(&filament); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := a.service.CreateFilament(&filament)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetFilaments returns all filaments
func (a *API) GetFilaments(c *gin.Context) {
	filaments := a.service.GetFilaments()
	c.JSON(http.StatusOK, filaments)
}

// CreatePrintJob creates a new print job
func (a *API) CreatePrintJob(c *gin.Context) {
	if a.redirectToLeader(c) {
		return
	}

	var job models.PrintJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := a.service.CreatePrintJob(&job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetPrintJobs returns all print jobs
func (a *API) GetPrintJobs(c *gin.Context) {
	// Get query parameters for filtering
	status := c.Query("status")
	
	printJobs := a.service.GetPrintJobs()
	
	// Filter by status if provided
	if status != "" {
		filtered := make([]*models.PrintJob, 0)
		for _, job := range printJobs {
			if strings.EqualFold(job.Status, status) {
				filtered = append(filtered, job)
			}
		}
		printJobs = filtered
	}
	
	c.JSON(http.StatusOK, printJobs)
}

// UpdatePrintJobStatus updates the status of a print job
func (a *API) UpdatePrintJobStatus(c *gin.Context) {
	if a.redirectToLeader(c) {
		return
	}

	jobID := c.Param("id")
	status := c.Query("status")

	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status parameter is required"})
		return
	}

	result, err := a.service.UpdatePrintJobStatus(jobID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// JoinNode adds a new node to the cluster
func (a *API) JoinNode(c *gin.Context) {
	var req struct {
		NodeID string `json:"node_id" binding:"required"`
		Addr   string `json:"addr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.service.JoinCluster(req.NodeID, req.Addr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetClusterInfo returns information about the Raft cluster
func (a *API) GetClusterInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"is_leader":     a.service.IsLeader(),
		"leader_address": a.service.GetLeaderAddress(),
	})
}