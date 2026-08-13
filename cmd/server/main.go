package main

import (
	"log"
	"net/http"

	"pscpt/internal/config"
	"pscpt/internal/db"
	"pscpt/internal/httpapp"
	"pscpt/internal/migrations"
)

func main() {
	cfg := config.Load()
	database, err := db.Open(cfg.DSN)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := migrations.Run(database); err != nil {
		log.Fatal(err)
	}
	log.Printf("pscpt listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, httpapp.App{DB: database, Cfg: cfg}.Handler()); err != nil {
		log.Fatal(err)
	}
}
