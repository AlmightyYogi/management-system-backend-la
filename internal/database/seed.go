package database

import (
	"fmt"

	"github.com/AlmightyOggy/management-system/internal/domain"
	"gorm.io/gorm"
)

func SeedMasterData(db *gorm.DB) {
	fmt.Println("Starting Master Data Seeding...")

	severities := []domain.MstSeverity{
		{Name: "1 - Emergency (Full Down)", Level: 1, Description: "Impact sangat tinggi, sistem down total", IsActive: true},
		{Name: "2 - Critical (Major Full Down)", Level: 2, Description: "Impact tinggi, sebagian besar fungsi terganggu", IsActive: true},
		{Name: "3 - Major (Partial Issue)", Level: 3, Description: "Impact sedang", IsActive: true},
		{Name: "4 - Minor", Level: 4, Description: "Impact rendah", IsActive: true},
	}
	seedMaster(db, "MstSeverity", severities)

	assignedTo := []domain.MstAssignedTo{
		{Name: "B2B Applications Operations", Description: "Tim B2B Apps Operations", IsActive: true},
		{Name: "Application Owner (L3)", Description: "Application Owner Level 3", IsActive: true},
	}
	seedMaster(db, "MstAssignedTo", assignedTo)

	scopes := []domain.MstScope{
		// Incident
		{Name: "Application Bug", Type: "Incident", Description: "", IsActive: true},
		{Name: "Infrastructure", Type: "Incident", Description: "", IsActive: true},
		{Name: "Human Error", Type: "Incident", Description: "", IsActive: true},
		{Name: "Deployment Issue", Type: "Incident", Description: "", IsActive: true},
		{Name: "Third Party", Type: "Incident", Description: "", IsActive: true},
		{Name: "Security Issue/Breach", Type: "Incident", Description: "", IsActive: true},
		{Name: "Unknown", Type: "Incident", Description: "", IsActive: true},
		// Request
		{Name: "User Management", Type: "Request", Description: "", IsActive: true},
		{Name: "Apps Improvement", Type: "Request", Description: "", IsActive: true},
		{Name: "Monitoring", Type: "Request", Description: "", IsActive: true},
		{Name: "Update/Patching", Type: "Request", Description: "", IsActive: true},
		{Name: "Licence/Certificate Management", Type: "Request", Description: "", IsActive: true},
		// Activity
		{Name: "Emergency", Type: "Activity", Description: "", IsActive: true},
		{Name: "Normal", Type: "Activity", Description: "", IsActive: true},
		{Name: "Standard", Type: "Activity", Description: "", IsActive: true},
		{Name: "Spotlight", Type: "Activity", Description: "", IsActive: true},
		// Common
		{Name: "Others", Type: "All", Description: "Lain-lain", IsActive: true},
	}
	seedMaster(db, "MstScope", scopes)

	impacts := []domain.MstImpact{
		{Name: "High", Description: "High Impact", IsActive: true},
		{Name: "Medium", Description: "Medium Impact", IsActive: true},
		{Name: "Low", Description: "Low Impact", IsActive: true},
		{Name: "No Impact", Description: "No Impact", IsActive: true},
	}
	seedMaster(db, "MstImpact", impacts)

	priorities := []domain.MstPriority{
		{Name: "1 - High", Level: 1, Description: "Priority High", IsActive: true},
		{Name: "2 - Medium", Level: 2, Description: "Priority Medium", IsActive: true},
		{Name: "3 - Low", Level: 3, Description: "Priority Low", IsActive: true},
	}
	seedMaster(db, "MstPriority", priorities)

	externalTeams := []domain.MstExternalTeam{
		{Name: "L3", Description: "Layer 3 For Developer"},
		{Name: "Kyndryl Windows Team", Description: "Kyndryl Windows Team"},
		{Name: "Kyndryl Linux Team", Description: "Kyndryl Linux Team"},
		{Name: "Kyndryl Network Team", Description: "Kyndryl Network Team"},
		{Name: "Kyndryl GCP Team", Description: "Kyndryl GCP Team"},
		{Name: "IOH Security Team", Description: "IOH Security Team"},
		{Name: "IOH Firewall Operation Team", Description: "IOH Firewall Operation Team"},
		{Name: "IOH Defensive Security Team", Description: "IOH Defensive Security Team"},
	}
	seedMaster(db, "MstExternalTeam", externalTeams)

	apps := []domain.MstApp{
		{Name: "B2B Portal", Description: "B2B Portal Application"},
		{Name: "SQA Portal", Description: "SQA Portal Application"},
		{Name: "SECM Portal", Description: "SECM Portal Application"},
		{Name: "DBEST", Description: "DBEST Application"},
		{Name: "IOT Middleware TransJakarta", Description: "IOT Middleware TransJakarta Application"},
		{Name: "MPR", Description: "MPR Application"},
		{Name: "SARAS", Description: "SARAS Application"},
		{Name: "My Dashboard", Description: "My Dashboard Application"},
		{Name: "PSSHUB", Description: "PSSHUB Application"},
	}
	seedMaster(db, "MstApp", apps)

	roles := []domain.MstRole{
		{Name: "admin"},
		{Name: "viewer"},
		{Name: "user"},
	}
	seedMaster(db, "MstRole", roles)

	fmt.Println("✅ Master Data Seeding completed successfully!")
}

func seedMaster(db *gorm.DB, modelName string, data interface{}) {
	result := db.CreateInBatches(data, 100)
	if result.Error != nil {
		fmt.Printf("Error seeding %s: %v\n", modelName, result.Error)
	} else {
		fmt.Printf("%s Seeder successfully (%d records)\n", modelName, result.RowsAffected)
	}
}