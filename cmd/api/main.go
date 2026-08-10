package main

import (
	"fmt"
	"log"
	"time"

	"github.com/AlmightyOggy/management-system/internal/config"
	"github.com/AlmightyOggy/management-system/internal/handler"
	"github.com/AlmightyOggy/management-system/internal/middleware"
	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/routes"
	"github.com/AlmightyOggy/management-system/internal/service"
	// "github.com/AlmightyOggy/management-system/internal/utils"

	// "github.com/AlmightyOggy/management-system/internal/database"

	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadConfig()
	cfg := config.GetConfig()

	config.ConnectDatabase()

	// err := utils.SendEmail(
	// 	"frayogi.sitorus@lintasarta.co.id",
	// 	"Test Email Brevo",
	// 	"<h2>Halo!</h2><p>Email berhasil dikirim dari aplikasi Go 🚀</p>",
	// )

	// if err != nil {
	// 	log.Println(err)
	// } else {
	// 	log.Println("Email berhasil dikirim")
	// }

	// database.AutoMigrate()

	// database.SeedMasterData(config.DB)

	masterRepo := repository.NewMasterRepository(config.DB)
	masterHandler := handler.NewMasterHandler(masterRepo)

	userRepo := repository.NewUserRepository(config.DB)
	reportRepo := repository.NewReportRepository(config.DB)
	externalTeamRepo := repository.NewReportExternalTeamRepository(config.DB)

	commentService := service.NewCommentService("storage/comments")
	commentHandler := handler.NewCommentHandler(commentService)

	userService := service.NewUserService(userRepo)
	reportService := service.NewReportService(reportRepo)
	externalTeamService := service.NewReportExternalTeamService(externalTeamRepo, reportRepo)

	userHandler := handler.NewUserHandler(userService)
	reportHandler := handler.NewReportHandler(reportService)
	externalTeamHandler := handler.NewReportExternalTeamHandler(externalTeamService)

	r := gin.New()
	r.MaxMultipartMemory = 50 << 20

	r.Use(middleware.CORS())

	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	routes.SetupRoutes(r, reportHandler, userHandler, externalTeamHandler, masterHandler, commentHandler)

	port := fmt.Sprintf(":%d", cfg.App.Port)
	log.Printf("%s started on http://localhost%s", cfg.App.Name, port)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("[Reminder Job] Checking open tickets...")
			if err := reportService.SendOpenTicketReminder(); err != nil {
				log.Printf("[Reminder Job] Error: %v", err)
			}
		}
	}()

	if err := r.Run(port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}