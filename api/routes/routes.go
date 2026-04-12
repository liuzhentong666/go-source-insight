package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/go-ai-study/api/handlers"
	"github.com/go-ai-study/api/middleware"
)

func RegisterRoutes(r *gin.Engine) {
	// 健康检查路由
	r.GET("/health", handlers.HealthCheck)

	// 用户相关路由
	userRoutes := r.Group("/api/v1/users")
	{
		userRoutes.POST("/register", handlers.RegisterUser)
		userRoutes.POST("/login", handlers.LoginUser)
	}

	// 受保护的路由 - 需要JWT认证
	protectedRoutes := r.Group("/api/v1")
	protectedRoutes.Use(middleware.AuthMiddleware())
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
		}

		// 分析相关路由
		analysisRoutes := protectedRoutes.Group("/analysis")
		{
			analysisRoutes.POST("/analyze", handlers.AnalyzeCode)
			analysisRoutes.GET("/:projectId", handlers.GetAnalysisResults)
		}
	}
}
