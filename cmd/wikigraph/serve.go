package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/api"
	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
)

var (
	serveHost       string
	servePort       int
	serveCORS       bool
	serveProduction bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the WikiGraph API server",
	Long: `Start the WikiGraph HTTP API server.

The server loads the graph into memory on startup for fast pathfinding queries.
It exposes REST endpoints for querying pages, finding paths, and exploring connections.

Examples:
  wikigraph serve
  wikigraph serve --port 3000
  wikigraph serve --host 0.0.0.0 --port 8080
  wikigraph serve --production`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&serveHost, "host", "", "host to bind to (default from config)")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 0, "port to listen on (default from config)")
	serveCmd.Flags().BoolVar(&serveCORS, "cors", true, "enable CORS")
	serveCmd.Flags().BoolVar(&serveProduction, "production", false, "enable production mode")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("received shutdown signal")
		cancel()
	}()

	// Open database
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// Initialize cache and fetcher
	c := cache.New(db)
	f := fetcher.New(fetcher.Config{
		RateLimit:      cfg.Scraper.RateLimit,
		RequestTimeout: cfg.Scraper.RequestTimeout,
		UserAgent:      cfg.Scraper.UserAgent,
		BaseURL:        cfg.Scraper.WikipediaAPIURL,
	})

	// Load graph into memory
	slog.Info("loading graph into memory...")
	loader := graph.NewLoader(c)
	g, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading graph: %w", err)
	}
	slog.Info("graph loaded",
		"nodes", g.NodeCount(),
		"edges", g.EdgeCount(),
	)

	// Build server config
	serverCfg := api.Config{
		Host:            cfg.API.Host,
		Port:            cfg.API.Port,
		EnableCORS:      cfg.API.EnableCORS,
		CORSOrigins:     cfg.API.CORSOrigins,
		ReadTimeout:     cfg.API.ReadTimeout,
		WriteTimeout:    cfg.API.WriteTimeout,
		ShutdownTimeout: cfg.API.ShutdownTimeout,
		RateLimit:       cfg.API.RateLimit,
		RateBurst:       cfg.API.RateBurst,
		Production:      cfg.API.Production,
	}

	// Override with command-line flags if provided
	if serveHost != "" {
		serverCfg.Host = serveHost
	}
	if servePort != 0 {
		serverCfg.Port = servePort
	}
	if cmd.Flags().Changed("cors") {
		serverCfg.EnableCORS = serveCORS
	}
	if cmd.Flags().Changed("production") {
		serverCfg.Production = serveProduction
	}

	// Create and start server
	server := api.New(g, c, f, serverCfg)

	fmt.Printf("Starting WikiGraph API server on http://%s:%d\n", serverCfg.Host, serverCfg.Port)
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("  GET  /health                        - Health check")
	fmt.Println("  GET  /api/v1/page/:title            - Get page links")
	fmt.Println("  GET  /api/v1/path?from=X&to=Y       - Find shortest path")
	fmt.Println("  GET  /api/v1/connections/:title     - Get N-hop neighborhood")
	fmt.Println("  POST /api/v1/crawl                  - Start background crawl")
	fmt.Println("\nPress Ctrl+C to stop")

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
