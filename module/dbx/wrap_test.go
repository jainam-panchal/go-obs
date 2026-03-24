package dbx

import (
	"testing"

	"github.com/jainam-panchal/go-obs/module/bootstrap"
	"github.com/jainam-panchal/go-obs/module/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testModel struct {
	ID   uint
	Name string
}

func TestWrapGORMRegistersCallbacksAndQueries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db = WrapGORM(db, &bootstrap.Runtime{Config: config.Config{ServiceName: "svc", DeploymentEnv: "dev"}})
	if db == nil {
		t.Fatal("WrapGORM returned nil")
	}

	if err := db.AutoMigrate(&testModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&testModel{Name: "a"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var got testModel
	if err := db.First(&got).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
}
