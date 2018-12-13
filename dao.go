package main

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"os"
)

type Person struct {
	ID        uint
	Name      string  `gorm:"size:50; not null"`
	Email     string  `gorm:"size:50; unique; not null"`
}

type Group struct {
	ID           uint
	PersonsCount uint    `gorm:"not null"`
	Link         string  `gorm:"unique; not null"`
	Closed       bool    `gorm:"default:false; not null"`
}

type GroupToPerson struct {
	ID       uint
	PersonID uint
	GroupID  uint
}

func GetDBClient() *gorm.DB {
	host := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	db, err := gorm.Open("postgres", "host=" + host + " port=6432 user=alexandr dbname=" + dbName + " password=" + os.
		Getenv("DB_PASS"))
	db.LogMode(true)
	if err != nil {
		panic(err)
	}
	return db
}
