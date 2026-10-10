package routes

import (
	"github.com/gin-gonic/gin"
	"go-ai-study/api/handlers"
	"go-ai-study/api/middleware"
)

func RegisterRoutes(r *gin.Engine, sourceHandlers ...*handlers.SourceHandler) {
	sourceHandler := &handlers.SourceHandler{}
	if len(sourceHandlers) > 0 && sourceHandlers[0] != nil {
		sourceHandler = sourceHandlers[0]
	}
	r.Use(func(c *gin.Context) { c.Set("token_service", sourceHandler.Tokens); c.Next() })
	// 健康检查路由
	r.GET("/health", handlers.HealthCheck)
	r.GET("/ready", sourceHandler.Ready)

	// 用户相关路由
	userRoutes := r.Group("/api/v1/users")
	{
		userRoutes.POST("/register", handlers.RegisterUser)
		userRoutes.POST("/login", handlers.LoginUser)
	}

	// 受保护的路由 - 需要JWT认证
	protectedRoutes := r.Group("/api/v1")
	protectedRoutes.Use(middleware.AuthMiddleware(sourceHandler.Tokens))
	{
		// 用户资料
		protectedRoutes.GET("/users/profile", handlers.GetUserProfile)

		// 项目相关路由
		projectRoutes := protectedRoutes.Group("/projects")
		{
			projectRoutes.POST("/", handlers.CreateProject)
			projectRoutes.GET("/", handlers.ListProjects)
			projectRoutes.GET("/:id", handlers.GetProject)
			projectRoutes.DELETE("/:id", handlers.DeleteProject)
			projectRoutes.POST("/:id/scan", sourceHandler.Scan)
			projectRoutes.GET("/:id/source", sourceHandler.SourceInfo)
			projectRoutes.POST("/:id/analyze", sourceHandler.Analyze)
			projectRoutes.POST("/:id/index", sourceHandler.Index)
			projectRoutes.POST("/:id/ask", sourceHandler.Ask)
		}
		protectedRoutes.GET("/jobs/:id", sourceHandler.GetJob)
		protectedRoutes.POST("/jobs/:id/cancel", sourceHandler.GetJob)

		// 分析相关路由
		analysisRoutes := protectedRoutes.Group("/analysis")
		{
			analysisRoutes.POST("/analyze", sourceHandler.AnalyzeCode)
			analysisRoutes.GET("/:projectId", handlers.GetAnalysisResults)
		}
	}
}
