package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/pmatteo/heapster/internal/commands/hash"
	"github.com/pmatteo/heapster/internal/commands/list"
	"github.com/pmatteo/heapster/internal/commands/set"
	"github.com/pmatteo/heapster/internal/config"
	"github.com/pmatteo/heapster/internal/server"
	"github.com/pmatteo/heapster/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}))

	ds := store.NewInMemoryStore(cfg.Store.MaxItems, logger)
	ds.RegisterFeature(list.NewFeature())
	ds.RegisterFeature(hash.NewFeature())
	ds.RegisterFeature(set.NewFeature(logger))

	srv := server.NewDefaultRespServer(ds, logger)

	log.Fatal(
		srv.Start(context.Background(), cfg.Server.Addr),
	)
}
