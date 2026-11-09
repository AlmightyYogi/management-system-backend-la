package routes

import (
	"github.com/AlmightyOggy/management-system/internal/handler"
	"github.com/AlmightyOggy/management-system/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, reportHandler *handler.ReportHandler, userHandler *handler.UserHandler, externalTeamHandler *handler.ReportExternalTeamHandler, masterHandler *handler.MasterHandler, commentHandler *handler.CommentHandler, vssHandler *handler.VSSHandler) {
	api := r.Group("/api")

	r.Static("/storage", "./storage/public")

	api.POST("/login", userHandler.Login)

	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware())

	nonGuest := protected.Group("/")
	nonGuest.Use(middleware.DenyRoles(middleware.RoleGuest))

	auths := nonGuest.Group("/register")
	{
		auths.POST("", userHandler.Register)
		auths.POST("/", userHandler.Register)
	}

	accounts := nonGuest.Group("/accounts")
	{
		accounts.GET("", userHandler.GetAllUsers)
		accounts.GET("/", userHandler.GetAllUsers)
		accounts.GET("/:uuid", userHandler.GetUserByUUID)
		accounts.PUT("/:uuid", userHandler.UpdateUser)
		accounts.POST("/change-password", userHandler.ChangePassword)
	}

	reports := nonGuest.Group("/reports")
	{
		reports.GET("", reportHandler.GetAllReports)
		reports.GET("/", reportHandler.GetAllReports)
		reports.GET("/export", reportHandler.ExportExcel)
		reports.GET("/export-count", reportHandler.ExportCount)
		reports.GET("/:uuid", reportHandler.GetReportByUUID)
		reports.GET("/:uuid/comments", commentHandler.GetComments)

		reportsWrite := reports.Group("")
		reportsWrite.Use(middleware.DenyRoles(middleware.RoleViewer))
		{
			reportsWrite.POST("", reportHandler.CreateReport)
			reportsWrite.POST("/", reportHandler.CreateReport)
			reportsWrite.PUT("/:uuid", reportHandler.UpdateReport)
			reportsWrite.DELETE("/:uuid", reportHandler.DeleteReport)
			reportsWrite.POST("/:uuid/restore", reportHandler.MarkRestored)
			reportsWrite.POST("/:uuid/toggle-handled", reportHandler.ToggleHandled)
			reportsWrite.POST("/:uuid/rca/export", reportHandler.ExportRCA)
			reportsWrite.POST("/:uuid/comments", commentHandler.AddComment)
			reportsWrite.DELETE("/:uuid/comments/:commentId", commentHandler.DeleteComment)
		}
	}

	externalTeams := nonGuest.Group("/report-external-teams")
	{
		externalTeams.GET("", externalTeamHandler.GetByReportID)
		externalTeams.GET("/", externalTeamHandler.GetByReportID)

		externalWrite := externalTeams.Group("")
		externalWrite.Use(middleware.DenyRoles(middleware.RoleViewer))
		{
			externalWrite.POST("", externalTeamHandler.CreateExternalTeam)
			externalWrite.POST("/", externalTeamHandler.CreateExternalTeam)
			externalWrite.PUT("/:id", externalTeamHandler.UpdateExternalTeam)
			externalWrite.DELETE("/:id", externalTeamHandler.DeleteExternalTeam)
		}
	}

	masters := nonGuest.Group("/master")
	{
		masters.GET("", masterHandler.GetAll)
		masters.GET("/", masterHandler.GetAll)
		masters.GET("/external-teams", masterHandler.GetExternalTeams)
		masters.GET("/scopes", masterHandler.GetScopes)
	}

	vss := protected.Group("/vss")
	{
		vss.GET("/delays", vssHandler.ListDelays)
		vss.GET("/history", vssHandler.ListHistory)
		vss.GET("/live", vssHandler.ListLive)
		vss.GET("/traffic", vssHandler.GetTraffic)
		vss.GET("/summary", vssHandler.GetDaySummary)
		vss.GET("/summary/export", vssHandler.ExportDayDelayExcel)
		vss.GET("/report/send", vssHandler.SendDailyReport)
		vss.GET("/devices/:id/stats", vssHandler.GetDeviceDelayStats)
		vss.GET("/devices/:id/history", vssHandler.GetDeviceDelayHistory)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}