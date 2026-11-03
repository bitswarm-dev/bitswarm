package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/auth"
	"bitswarm/server/internal/backup"
	"bitswarm/server/internal/buildinfo"
	"bitswarm/server/internal/catalog"
	"bitswarm/server/internal/config"
	"bitswarm/server/internal/db"
	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/engine"
	"bitswarm/server/internal/eventlog"
	"bitswarm/server/internal/events"
	"bitswarm/server/internal/health"
	"bitswarm/server/internal/hooks"
	httpserver "bitswarm/server/internal/http"
	"bitswarm/server/internal/logger"
	"bitswarm/server/internal/maintenance"
	"bitswarm/server/internal/pathcheck"
	"bitswarm/server/internal/renames"
	"bitswarm/server/internal/schedules"
	"bitswarm/server/internal/settings"
	"bitswarm/server/internal/tokens"
	"bitswarm/server/internal/torrents"
	"bitswarm/server/internal/updates"
)

var version = buildinfo.Version

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(logger.Options{Level: logger.ParseLevel(cfg.LogLevel), Format: cfg.LogFormat})

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	settings.SetDownloadDir(cfg.DownloadDir)
	kv := docstore.NewKV(conn)
	bus := events.NewBus()
	xlog := eventlog.NewService(eventlog.NewCollection(conn))
	bus.Subscribe(xlog)

	catalogSvc, err := catalog.NewService(kv)
	fatal(log, err)
	settingsSvc, err := settings.NewService(kv, bus, nil)
	fatal(log, err)
	authSvc, err := auth.NewService(kv, settingsSvc, xlog)
	fatal(log, err)
	settingsSvc.SetPasswords(authSvc)
	settingsSvc.SetPathCheck(pathcheck.Ensure)

	torrentSvc := torrents.NewService(torrents.NewRepository(conn), settingsSvc, catalogSvc, bus, xlog)
	torrentSvc.SetPathCheck(pathcheck.Ensure)
	if !cfg.Demo {
		ps := settingsSvc.Get()
		eng, err := engine.New(engine.Config{
			DataDir: filepath.Join(filepath.Dir(cfg.DBPath), "engine"), Port: ps.Port, DHT: ps.Dht, Upnp: ps.Upnp,
			Encryption: ps.Encryption, Version: version,
		})
		if err != nil {
			log.Error("failed to start the BitTorrent engine", "error", err)
			os.Exit(1)
		}
		defer eng.Close()
		torrentSvc.SetEngine(eng, func() float64 { return torrents.DiskFree(cfg.DownloadDir) })
	}

	renamePresets, renameLog := renames.NewCollections(conn)
	renameSvc := renames.NewService(renamePresets, renameLog, torrentSvc, bus)
	torrentSvc.SetRenamer(renameSvc)

	schedRules, schedLog := schedules.NewCollections(conn)
	schedSvc := schedules.NewService(schedRules, schedLog, torrentSvc, bus)
	schedSvc.SetPathCheck(pathcheck.Ensure)
	schedSvc.SetDelay(func() time.Duration { return time.Duration(settingsSvc.Get().SearchDelay) * time.Second })
	renameSvc.SetUsers(schedSvc.UsersOf)
	renameSvc.OnDelete(schedSvc.ClearPreset)
	go schedSvc.Loop(ctx)

	maintTasks, maintLog := maintenance.NewCollections(conn)
	maintSvc := maintenance.NewService(maintTasks, maintLog, torrentSvc, bus)
	fatal(log, maintSvc.Seed(ctx))
	maintSvc.AddPruner(maintenance.AdaptPruner("Schedules", schedSvc.Old, schedSvc.Remove, func(e schedules.OldEntry) maintenance.OldEntry {
		return maintenance.OldEntry{ID: e.ID, Label: e.Label, T: e.T}
	}))
	maintSvc.AddPruner(maintenance.AdaptPruner("Renames", renameSvc.Old, renameSvc.Remove, func(e renames.OldEntry) maintenance.OldEntry {
		return maintenance.OldEntry{ID: e.ID, Label: e.Label, T: e.T}
	}))
	maintSvc.AddPruner(maintSvc)
	go maintSvc.Loop(ctx)

	hookItems, hookDeliveries := hooks.NewCollections(conn)
	hookSvc := hooks.NewService(hookItems, hookDeliveries, xlog, cfg.AllowCommands)
	hookSvc.SetSamples(hooks.Samples{
		Torrent: func(ctx context.Context) map[string]any {
			if ts, err := torrentSvc.Snapshot(ctx); err == nil && len(ts) > 0 {
				return torrents.Info(ts[0])
			}
			return nil
		},
		Schedule: func(ctx context.Context) (int64, string, []string) {
			if rs, err := schedSvc.List(ctx); err == nil && len(rs) > 0 {
				return rs[0].ID, rs[0].Name, []string{rs[0].URL}
			}
			return 1, "My schedule", []string{}
		},
	})
	catalogSvc.SetLog(xlog.Add)
	for _, o := range []catalog.Owner{torrentSvc, schedSvc, renameSvc, maintSvc, hookSvc} {
		catalogSvc.AddOwner(o)
	}
	bus.Subscribe(hookSvc)
	if cfg.AllowCommands {
		xlog.Add("warn", "Hooks", "Command hooks are on (SWARM_ALLOW_COMMANDS=1). They run on this machine with Bitswarm’s permissions.")
	}

	if cfg.Demo {
		fatal(log, torrentSvc.Seed(ctx))
		fatal(log, renameSvc.Seed(ctx))
		fatal(log, schedSvc.Seed(ctx, 1, cfg.DownloadDir))
		fatal(log, hookSvc.Seed(ctx))
		xlog.Add("info", "Torrents", "Loaded sample torrents and rename presets (SWARM_DEMO=1)")
	}
	go torrentSvc.Run(ctx)

	set := settingsSvc.Get()
	xlog.Add("info", "App", "Bitswarm "+version+" started")
	xlog.Addf("info", "Network", "Listening for peers on port %d%s", set.Port, map[bool]string{true: ", forwarded with UPnP", false: ""}[set.Upnp])
	if set.Dht {
		xlog.Add("info", "Network", "DHT is on")
	} else {
		xlog.Add("warn", "Network", "DHT is off")
	}

	var feed updates.Feed
	switch {
	case cfg.UpdateFeed != "":
		feed = updates.URLFeed(cfg.UpdateFeed)
	case cfg.Demo:
		feed = updates.SampleFeed()
	}
	updateSvc := updates.NewService(version, feed, feed != nil && cfg.UpdateFeed == "", bus)
	go func() {
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
			if feed != nil && settingsSvc.Get().AutoUpdate {
				if st := updateSvc.Check(ctx); st.State == "available" {
					xlog.Addf("warn", "Updates", "Bitswarm %s is available. You’re running %s.", st.Latest.Version, version)
				}
			}
		}
	}()

	tokenSvc := tokens.NewService(tokens.NewCollection(conn), xlog)
	backupSvc := backup.NewService()
	backupSvc.Settings, backupSvc.Catalog, backupSvc.Renames, backupSvc.Schedules = settingsSvc, catalogSvc, renameSvc, schedSvc
	backupSvc.Maintenance, backupSvc.Hooks, backupSvc.Log = maintSvc, hookSvc, xlog

	authH := auth.NewHandler(authSvc)
	authH.SetTokens(tokenSvc.Verify, tokens.Allowed)
	healthH := health.NewHandler(health.NewService(health.NewRepository(conn), log))
	handlers := []interface{ RegisterRoutes(fiber.Router) }{
		settings.NewHandler(settingsSvc), catalog.NewHandler(catalogSvc), torrents.NewHandler(torrentSvc),
		eventlog.NewHandler(xlog), renames.NewHandler(renameSvc), schedules.NewHandler(schedSvc), maintenance.NewHandler(maintSvc), hooks.NewHandler(hookSvc), tokens.NewHandler(tokenSvc), backup.NewHandler(backupSvc), updates.NewHandler(updateSvc), healthH,
	}

	app := httpserver.New(log, func(r fiber.Router) {
		r.Use("/api/v1", func(c *fiber.Ctx) error {
			c.Path("/api" + strings.TrimPrefix(c.Path(), "/api/v1"))
			return c.RestartRouting()
		})
		api := r.Group("/api")
		authH.RegisterPublic(api)
		api.Use(authH.Require())
		for _, h := range handlers {
			h.RegisterRoutes(api)
		}
	})
	go func() {
		<-ctx.Done()
		_ = app.Shutdown()
	}()

	log.Info("starting server", "port", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func fatal(log *logger.Logger, err error) {
	if err != nil {
		log.Error("startup failed", "error", err)
		os.Exit(1)
	}
}
