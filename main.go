// Package main is the desktop entry point for the Athena application.
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/santaniello/athena/internal/application/auth"
	"github.com/santaniello/athena/internal/application/onboarding"
	"github.com/santaniello/athena/internal/infrastructure/athenahome"
	"github.com/santaniello/athena/internal/infrastructure/configfile"
	"github.com/santaniello/athena/internal/infrastructure/openrouter"
	"github.com/santaniello/athena/internal/infrastructure/profilefile"
	"github.com/santaniello/athena/internal/infrastructure/session"
	"github.com/santaniello/athena/internal/infrastructure/sqlite"
	"github.com/santaniello/athena/internal/interfaces/desktop"
)

// frontend/ must stay inside this module (no nested go.mod) for this
// directive to work — go:embed cannot reach into a different Go module.
// That's why the node_modules-vs-Go-tooling firewall lives one level
// deeper, at frontend/node_modules/go.mod, instead of here.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dbPath, err := athenahome.File("athena.db")
	if err != nil {
		log.Fatalf("resolving database path: %v", err)
	}
	sessionPath, err := athenahome.File("session.json")
	if err != nil {
		log.Fatalf("resolving session path: %v", err)
	}
	profilePath, err := athenahome.File("profile.json")
	if err != nil {
		log.Fatalf("resolving profile path: %v", err)
	}
	configPath, err := athenahome.File("config.yaml")
	if err != nil {
		log.Fatalf("resolving config path: %v", err)
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("closing database: %v", err)
		}
	}()

	accounts := sqlite.NewAccountRepository(db)
	sessions := session.NewStore(sessionPath)
	authService := auth.NewService(accounts, sessions)

	profiles := profilefile.NewStore(profilePath)
	configStore := configfile.NewStore(configPath)
	keyValidator := openrouter.NewValidator("")
	onboardingService := onboarding.NewService(profiles, configStore, keyValidator)

	app := desktop.NewApp(authService, sessions, onboardingService, profiles, configStore)

	err = wails.Run(&options.App{
		Title:  "Athena",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
