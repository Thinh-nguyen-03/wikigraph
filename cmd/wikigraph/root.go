package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/config"
)

var (
	cfgFile string
	verbose bool
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "wikigraph",
	Short: "Wikipedia knowledge graph builder",
	Long:  `WikiGraph crawls Wikipedia articles, extracts links, and builds a knowledge graph for exploration and pathfinding.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogging()

		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func setupLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}
