package app

import (
	"database/sql"
	"fmt"
	"log"
)

type App struct {
	DB *sql.DB
}

func (a *App) CreateConntection() {
	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		UNAMEDB, PASSDB, HOSTDB, DBNAME)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	a.DB = db
}

func (a *App) Run() {
	StartConnectMQTT(a)
}
