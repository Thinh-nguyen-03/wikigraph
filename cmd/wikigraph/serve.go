package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/api"
	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
	"github.com/Thinh-nguyen-03/wikigraph/internal/neostore"
)

var (
	serveHost         string
	servePort         int
	serveCORS         bool
	serveProduction   bool
	serveForceRebuild bool
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
	serveCmd.Flags().BoolVar(&serveForceRebuild, "rebuild-cache", false, "force rebuild graph cache from database")
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

	// Determine graph cache path
	cachePath := cfg.Graph.CachePath
	if cachePath == "" {
		// Default to same directory as database
		dbDir := filepath.Dir(cfg.Database.Path)
		cachePath = filepath.Join(dbDir, "graph.cache")
	}

	// Create GraphService with background loading
	graphServiceCfg := api.GraphServiceConfig{
		CachePath:       cachePath,
		MaxCacheAge:     cfg.Graph.MaxCacheAge,
		RefreshInterval: cfg.Graph.RefreshInterval,
		ForceRebuild:    serveForceRebuild || cfg.Graph.ForceRebuild,
	}
	graphService := api.NewGraphService(c, graphServiceCfg)

	// Start background graph loading (returns immediately)
	graphService.Start(ctx)
	defer graphService.Stop()

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

	// Initialize Neo4j client if enabled
	slog.Info("neo4j config check", "enabled", cfg.Neo4j.Enabled, "uri", cfg.Neo4j.URI)
	var neo4jClient *neostore.Client
	var neo4jSyncer *neostore.Syncer
	if cfg.Neo4j.Enabled {
		slog.Info("connecting to Neo4j", "uri", cfg.Neo4j.URI)
		client, err := neostore.NewClient(neostore.Config{
			URI:                          cfg.Neo4j.URI,
			Username:                     cfg.Neo4j.Username,
			Password:                     cfg.Neo4j.Password,
			MaxConnectionPoolSize:        cfg.Neo4j.MaxConnectionPoolSize,
			ConnectionAcquisitionTimeout: cfg.Neo4j.ConnectionAcquisitionTimeout,
		})
		if err != nil {
			slog.Warn("failed to create Neo4j client, falling back to in-memory graph", "error", err)
		} else {
			// Verify connectivity
			if err := client.VerifyConnectivity(ctx); err != nil {
				slog.Warn("Neo4j connectivity check failed, falling back to in-memory graph", "error", err)
				client.Close(ctx)
			} else {
				neo4jClient = client
				neo4jSyncer = neostore.NewSyncer(neo4jClient, db.DB)
				slog.Info("connected to Neo4j successfully")
				defer neo4jClient.Close(ctx)
			}
		}
	}

	// Create and start server
	var server *api.Server
	if neo4jClient != nil {
		server = api.NewWithNeo4j(graphService, c, f, neo4jClient, neo4jSyncer, serverCfg)
	} else {
		server = api.NewWithGraphService(graphService, c, f, serverCfg)
	}

	fmt.Printf("Starting WikiGraph API server on http://%s:%d\n", serverCfg.Host, serverCfg.Port)
	if neo4jClient != nil {
		fmt.Println("\nNeo4j backend enabled - queries served from graph database")
	} else {
		fmt.Println("\nGraph loading in background - server is immediately available")
		fmt.Println("Note: Path queries will return 503 until graph is ready")
	}
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("  GET  /health                        - Health check (shows graph status)")
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
