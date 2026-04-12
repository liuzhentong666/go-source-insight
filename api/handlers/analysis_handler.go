package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-ai-study/api/analysis"
	"github.com/go-ai-study/api/database"
	"github.com/go-ai-study/api/models"
	"gorm.io/gorm"
)

// AnalyzeCode 分析代码
func AnalyzeCode(c *gin.Context) {
	var req models.AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	var project models.Project
	result := database.DB.First(&project, req.ProjectID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if project.OwnerID != userID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to analyze this project"})
		return
	}

	// 创建分析记录
	analysisRecord := models.Analysis{
		ProjectID: req.ProjectID,
		Status:    "running",
	}
	result = database.DB.Create(&analysisRecord)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create analysis record"})
		return
	}

	// 异步执行分析
	go performAnalysis(analysisRecord.ID, req.Code)

	c.JSON(http.StatusAccepted, gin.H{
		"message":     "Analysis started",
		"analysis_id": analysisRecord.ID,
		"status":      "running",
	})
}

// performAnalysis 执行实际的代码分析
func performAnalysis(analysisID uint, code string) {
	// 执行分析
	result, err := analysis.PerformAnalysis(code)
	if err != nil {
		// 更新为失败状态
		database.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
			"status": "failed",
			"result": err.Error(),
		})
		return
	}

	// 更新分析记录
	database.DB.Model(&models.Analysis{}).Where("id = ?", analysisID).Updates(map[string]interface{}{
		"status": "completed",
		"result": result.ToJSON(),
	})
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
