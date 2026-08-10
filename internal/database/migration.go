package database

import (
	"log"

	"github.com/AlmightyOggy/management-system/internal/config"
	"github.com/AlmightyOggy/management-system/internal/domain"
)

func AutoMigrate() {
	db := config.DB

	err := db.AutoMigrate(
		&domain.User{},
		&domain.Report{},
		&domain.MstSeverity{},
		&domain.MstPriority{},
		&domain.MstAssignedTo{},
		&domain.MstScope{},
		&domain.MstImpact{},
		&domain.MstApp{},
		&domain.MstExternalTeam{},
		&domain.MstRole{},
		&domain.ReportExternalTeam{},
	)

	if err != nil {
		log.Fatal("Migration failed:", err)
	}

	log.Println("Auto Migration completed successfully")
}