// Controller for job related endpoints
package handlers

import (
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type JobHandler struct {
	jobService *services.JobService
}

func NewJobHandler(jobService *services.JobService) *JobHandler {
	return &JobHandler{
		jobService: jobService,
	}
}

func (h *JobHandler) CreateJob(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.jobService.CreateJob(c, &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

// GetJobs returns all jobs for a user
func (h *JobHandler) GetJobs(c *gin.Context) {
	userID := c.GetString("user_id")
	workspaceID := c.Query("workspace_id")
	if userID == "" || workspaceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse pagination parameters
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")

	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 1 {
		pageInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}

	jobs, err := h.jobService.GetJobs(c, &services.GetJobsRequest{
		UserId:      userID,
		WorkspaceId: workspaceID,
		Page:        pageInt,
		Limit:       limitInt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, jobs)
}

func (h *JobHandler) GetJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get pagination parameters for tasks
	taskPage := c.DefaultQuery("task_page", "1")
	taskLimit := c.DefaultQuery("task_limit", "10")

	taskPageInt, err := strconv.Atoi(taskPage)
	if err != nil || taskPageInt < 1 {
		taskPageInt = 1
	}
	taskLimitInt, err := strconv.Atoi(taskLimit)
	if err != nil || taskLimitInt < 1 {
		taskLimitInt = 10
	}

	resp, err := h.jobService.GetJob(c, &services.GetJobRequest{
		Id:        jobID,
		UserId:    userID,
		TaskPage:  taskPageInt,
		TaskLimit: taskLimitInt,
	})
	// Create response with job and paginated tasks
	response := gin.H{
		"job":   resp.Job,
		"tasks": resp.Tasks,
	}

	c.JSON(http.StatusOK, response)
}

func (h *JobHandler) DeleteJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.jobService.DeleteJob(c, &services.DeleteJobRequest{
		Id:     jobID,
		UserId: userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
}

func (h *JobHandler) PauseJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.jobService.PauseJob(c, uuid.MustParse(jobID), uuid.MustParse(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job paused successfully"})
}

func (h *JobHandler) ResumeJob(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.jobService.ResumeJob(c, uuid.MustParse(jobID), uuid.MustParse(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job resumed successfully"})
}
