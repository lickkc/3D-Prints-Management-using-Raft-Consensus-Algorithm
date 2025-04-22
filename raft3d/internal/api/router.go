package api

import (
	"github.com/gin-gonic/gin"
	"github.com/JyiaJain/raft3d/internal/service"
)

// SetupRouter configures the HTTP router
func SetupRouter(svc *service.Service) *gin.Engine {
	r := gin.Default()
	api := NewAPI(svc)

	// API version group
	v1 := r.Group("/api/v1")
	{
		// Printers
		v1.POST("/printers", api.CreatePrinter)
		v1.GET("/printers", api.GetPrinters)

		// Filaments
		v1.POST("/filaments", api.CreateFilament)
		v1.GET("/filaments", api.GetFilaments)

		// Print Jobs
		v1.POST("/print_jobs", api.CreatePrintJob)
		v1.GET("/print_jobs", api.GetPrintJobs)
		v1.POST("/print_jobs/:id/status", api.UpdatePrintJobStatus)

		// Cluster management
		v1.POST("/cluster/join", api.JoinNode)
		v1.GET("/cluster/info", api.GetClusterInfo)
	}

	return r
}