package app

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"
)

type App struct {
	DB            *sql.DB
	AODSArchiveDB *sql.DB
}

func (a *App) CreateConnection() {
	connStr := buildPostgresConnString(UNAMEDB, PASSDB, HOSTDB, DBPORT, DBNAME)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connect To DB")

	a.DB = db

	a.createAODSArchiveConnection()
}

func (a *App) Run() {
	StartConsumeKafka(a)
}

func (a *App) createAODSArchiveConnection() {
	if strings.TrimSpace(AODSArchiveDBHost) == "" ||
		strings.TrimSpace(AODSArchiveDBName) == "" ||
		strings.TrimSpace(AODSArchiveDBUser) == "" {
		log.Println("AODS archive DB disabled (missing AODS_ARCHIVE_DB_* settings)")
		return
	}

	connStr := buildPostgresConnString(
		AODSArchiveDBUser,
		AODSArchiveDBPassword,
		AODSArchiveDBHost,
		AODSArchiveDBPort,
		AODSArchiveDBName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Printf("AODS archive DB disabled (connect error): %v", err)
		return
	}

	if err := db.Ping(); err != nil {
		log.Printf("AODS archive DB disabled (ping error): %v", err)
		_ = db.Close()
		return
	}

	log.Println("Connect To AODS archive DB")
	a.AODSArchiveDB = db
}

func buildPostgresConnString(user, password, host, port, dbName string) string {
	addr := host
	if port != "" && !strings.Contains(host, ":") {
		addr = fmt.Sprintf("%s:%s", host, port)
	}

	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		user, password, addr, dbName)
}
