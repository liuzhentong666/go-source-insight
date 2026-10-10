package handlers

import (
	"github.com/gin-gonic/gin"
	"go-ai-study/api/database"
	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

// AnalyzeCode preserves the legacy analysis_id and history contract while all
// execution uses the durable bounded job engine.
func (h *SourceHandler) AnalyzeCode(c *gin.Context) {
	var req *models.AnalysisRequest
	if err := decodeRequest(c, &req, false); err != nil || req == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid analysis request"})
		return
	}
	owner, ok := c.Get("user_id")
	ownerID, typed := owner.(uint)
	if !ok || !typed || ownerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if h.Service == nil {
		h.fail(c, fault.ErrStorage)
		return
	}
	job, err := h.Service.SubmitLegacy(c.Request.Context(), ownerID, req.ProjectID, req.Code)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Location", "/api/v1/jobs/"+job.ID)
	c.JSON(http.StatusAccepted, gin.H{"message": "Analysis started", "analysis_id": job.LegacyAnalysisID, "job_id": job.ID, "status": job.Status})
}

// GetAnalysisResults 获取分析结果
func GetAnalysisResults(c *gin.Context) {
	// 获取项目ID
	projectID := c.Param("projectId")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// 检查数据库连接
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection not initialized"})
		return
	}

	// 验证项目存在且属于当前用户
	id, err := strconv.ParseUint(projectID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var project models.Project
	result := database.DB.First(&project, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if project.OwnerID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view this project's analysis"})
		return
	}

	// 查询分析结果
	var analyses []models.Analysis
	result = database.DB.Where("project_id = ?", id).Order("created_at desc").Find(&analyses)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analysis results"})
		return
	}

	c.JSON(http.StatusOK, analyses)
}
