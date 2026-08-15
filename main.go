// Package main is the desktop entry point for the Athena application.
package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"log"
	"os"
	"os/exec"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

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
// Matched by our own PID (via `wmctrl -lp`), not by title or WM_CLASS: a
// title match is ambiguous on a machine developing this app, where an
// editor/IDE window legitimately has "athena" in its title too, and
// options.Linux.ProgramName doesn't help — Wails calls g_set_prgname after
// the window is already created, too late to affect its WM_CLASS.
//
// OnStartup can fire before the window manager has finished mapping/
// registering the window (observed directly: under a loaded system, the
// window took ~65s to show up in `wmctrl -lp` instead of the usual ~8s), so
// a single attempt isn't reliable — retry for a few seconds. No-op (logged)
// if wmctrl isn't installed, or if the window still isn't found once the
// retry budget runs out.
func activateLinuxWindow() {
	if _, err := exec.LookPath("wmctrl"); err != nil {
		log.Printf("wmctrl not installed, cannot bring the window to the foreground: %v", err)
		return
	}

	pid := strconv.Itoa(os.Getpid())
	for range 20 {
		if activateWindowByPID(pid) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// activateWindowByPID looks up the window owned by the given PID via
// `wmctrl -lp` and, if found, asks the window manager to raise and focus
// it. Returns false if the window isn't found yet, so the caller can retry.
func activateWindowByPID(pid string) bool {
	out, err := exec.Command("wmctrl", "-lp").Output()
	if err != nil {
		log.Printf("listing windows via wmctrl: %v", err)
		return false
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		// Columns: <window id> <desktop> <pid> <client machine> <title...>
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[2] != pid {
			continue
		}
		if err := exec.Command("wmctrl", "-i", "-a", fields[0]).Run(); err != nil {
			log.Printf("activating window via wmctrl: %v", err)
		}
		return true
	}
	return false
}
