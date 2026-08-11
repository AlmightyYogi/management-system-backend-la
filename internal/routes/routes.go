package routes

import (
	"github.com/AlmightyOggy/management-system/internal/handler"
	"github.com/AlmightyOggy/management-system/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, reportHandler *handler.ReportHandler, userHandler *handler.UserHandler, externalTeamHandler *handler.ReportExternalTeamHandler, masterHandler *handler.MasterHandler, commentHandler *handler.CommentHandler) {
	api := r.Group("/api")

	r.Static("/storage", "./storage/public")

	api.POST("/login", userHandler.Login)

	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware())

	auths := protected.Group("/register")
	{
		auths.POST("", userHandler.Register)
		auths.POST("/", userHandler.Register)
	}

	accounts := protected.Group("/accounts")
	{
		accounts.GET("", userHandler.GetAllUsers)
		accounts.GET("/", userHandler.GetAllUsers)
		accounts.GET("/:uuid", userHandler.GetUserByUUID)
		accounts.PUT("/:uuid", userHandler.UpdateUser)
	}

	reports := protected.Group("/reports")
	{
		reports.POST("", reportHandler.CreateReport)
		reports.POST("/", reportHandler.CreateReport)
		reports.GET("", reportHandler.GetAllReports)
		reports.GET("/", reportHandler.GetAllReports)
		reports.GET("/export", reportHandler.ExportExcel)
		reports.GET("/export-count", reportHandler.ExportCount)
		reports.GET("/:uuid", reportHandler.GetReportByUUID)
		reports.PUT("/:uuid", reportHandler.UpdateReport)
		reports.POST("/:uuid/restore", reportHandler.MarkRestored)
		reports.POST("/:uuid/toggle-handled", reportHandler.ToggleHandled)
		reports.POST("/:uuid/rca/export", reportHandler.ExportRCA)

		reports.GET("/:uuid/comments", commentHandler.GetComments)
		reports.POST("/:uuid/comments", commentHandler.AddComment)
		reports.DELETE("/:uuid/comments/:commentId", commentHandler.DeleteComment)
	}

	externalTeams := protected.Group("/report-external-teams")
	{
		externalTeams.GET("", externalTeamHandler.GetByReportID)
		externalTeams.GET("/", externalTeamHandler.GetByReportID)
		externalTeams.POST("", externalTeamHandler.CreateExternalTeam)
		externalTeams.POST("/", externalTeamHandler.CreateExternalTeam)
		externalTeams.PUT("/:id", externalTeamHandler.UpdateExternalTeam)
		externalTeams.DELETE("/:id", externalTeamHandler.DeleteExternalTeam)
	}

	masters := protected.Group("/master")
	{
		masters.GET("", masterHandler.GetAll)
		masters.GET("/", masterHandler.GetAll)
		masters.GET("/external-teams", masterHandler.GetExternalTeams)
		masters.GET("/scopes", masterHandler.GetScopes)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}