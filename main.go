// Package main is the desktop entry point for the Athena application.
package main

import (
	"context"
	"embed"
	"log"
	"os/exec"
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/santaniello/athena/internal/application/auth"
	"github.com/santaniello/athena/internal/application/folder"
	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	"github.com/santaniello/athena/internal/application/onboarding"
	"github.com/santaniello/athena/internal/application/study"
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

	cfg, cfgErr := configStore.Load()
	if cfgErr != nil {
		log.Printf("loading config: %v", cfgErr)
	}
	usageRepo := sqlite.NewUsageRepository(db)
	llmClient := openrouter.NewClient("", cfg.OpenRouterKey, usageRepo)
	studySessions := sqlite.NewSessionRepository(db)
	studyMessages := sqlite.NewMessageRepository(db)
	folders := sqlite.NewFolderRepository(db)
	studyService := study.NewService(studySessions, studyMessages, llmClient, profiles, folders)
	folderService := folder.NewService(folders, studySessions)
	knowledgeItems := sqlite.NewKnowledgeRepository(db)
	knowledgeService := applicationknowledge.NewService(knowledgeItems, studySessions, studyMessages, llmClient, configStore)

	app := desktop.NewApp(authService, sessions, onboardingService, profiles, configStore, studyService, folderService, knowledgeService, llmClient)

	err = wails.Run(&options.App{
		Title:            "Athena",
		Width:            1024,
		Height:           768,
		WindowStartState: options.Normal,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// Some Linux window managers open new GTK windows minimised or
		// unfocused regardless of WindowStartState — force it visible and
		// centred every launch. Bringing it to the foreground needs a
		// separate workaround; see activateLinuxWindow.
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			wailsruntime.WindowUnminimise(ctx)
			wailsruntime.WindowCenter(ctx)
			if goruntime.GOOS == "linux" {
				go activateLinuxWindow()
			}
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// activateLinuxWindow asks the window manager to raise and focus the Athena
// window. GTK's own focus request (gtk_window_present, which backs Wails'
// WindowUnminimise/WindowCenter) gets silently downgraded to a "demands
// attention" taskbar flash by focus-stealing prevention on GNOME/Cinnamon-
// family window managers when the app was launched from a terminal rather
// than a desktop launcher — confirmed on Cinnamon/Muffin, where toggling
// always-on-top produced the same flash instead of an actual focus change.
// wmctrl's activation request carries the EWMH "pager" source indicator,
// which these window managers trust instead.
//
// Matched by the window's exact title ("Athena", set via options.App.Title),
// not by PID: an earlier PID-based version parsed `wmctrl -lp` and fed the
// resulting window ID back into a second wmctrl call, which gosec's G204
// flags on any exec.Command argument that isn't a string literal — with no
// exception for values that were validated first, since the check is purely
// syntactic. A literal title keeps every exec.Command argument constant. The
// trade-off: on a machine developing this app, a window from another
// process (e.g. an editor/IDE tab) that also has the exact title "Athena"
// would be activated instead — an unlikely collision, and not destructive
// even if it happens.
//
// OnStartup can fire before the window manager has finished mapping/
// registering the window (observed directly: under a loaded system, the
// window took ~65s to show up instead of the usual ~8s), so a single
// attempt isn't reliable — retry for a few seconds. No-op (logged) if
// wmctrl isn't installed, or if the window still isn't found once the retry
// budget runs out.
func activateLinuxWindow() {
	if _, err := exec.LookPath("wmctrl"); err != nil {
		log.Printf("wmctrl not installed, cannot bring the window to the foreground: %v", err)
		return
	}

	for range 20 {
		if exec.Command("wmctrl", "-F", "-a", "Athena").Run() == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	log.Print("could not find the Athena window via wmctrl within the retry budget")
}
