package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
)

var (
	pathMaxDepth    int
	bidirectional   bool
	outputFormat    string
)

var pathCmd = &cobra.Command{
	Use:   "path <from> <to>",
	Short: "Find the shortest path between two Wikipedia pages",
	Long: `Find the shortest path between two Wikipedia pages.

The pages must already be in the local database. Use 'wikigraph fetch'
to crawl pages first.

Examples:
  wikigraph path "Albert Einstein" "Physics"
  wikigraph path "Go (programming language)" "Python" --max-depth 10
  wikigraph path "Cat" "Dog" --bidirectional`,
	Args: cobra.ExactArgs(2),
	RunE: runPath,
}

func init() {
	rootCmd.AddCommand(pathCmd)

	pathCmd.Flags().IntVarP(&pathMaxDepth, "max-depth", "d", 6, "maximum path length to search")
	pathCmd.Flags().BoolVarP(&bidirectional, "bidirectional", "b", false, "use bidirectional search")
	pathCmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "output format: text, json")
}

type pathOutput struct {
	Found      bool     `json:"found"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Path       []string `json:"path,omitempty"`
	Hops       int      `json:"hops"`
	Explored   int      `json:"explored"`
	DurationMs int64    `json:"duration_ms"`
	Algorithm  string   `json:"algorithm"`
	Nodes      int      `json:"nodes"`
	Edges      int      `json:"edges"`
}

func runPath(cmd *cobra.Command, args []string) error {
	from, to := args[0], args[1]

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	c := cache.New(db)
	loader := graph.NewLoader(c)

	loadStart := time.Now()
	g, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading graph: %w", err)
	}
	loadDuration := time.Since(loadStart)

	if g.NodeCount() == 0 {
		return fmt.Errorf("graph is empty - use 'wikigraph fetch' to crawl pages first")
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Loaded %d nodes, %d edges in %s\n",
			g.NodeCount(), g.EdgeCount(), loadDuration.Truncate(time.Millisecond))
	}

	searchStart := time.Now()
	var result graph.PathResult
	algorithm := "bfs"

	if bidirectional {
		algorithm = "bidirectional"
		result = g.FindPathBidirectional(from, to)
	} else {
		result = g.FindPathWithLimit(from, to, pathMaxDepth)
	}
	searchDuration := time.Since(searchStart)

	out := pathOutput{
		Found:      result.Found,
		From:       from,
		To:         to,
		Path:       result.Path,
		Hops:       result.Hops,
		Explored:   result.Explored,
		DurationMs: searchDuration.Milliseconds(),
		Algorithm:  algorithm,
		Nodes:      g.NodeCount(),
		Edges:      g.EdgeCount(),
	}

	switch outputFormat {
	case "json":
		return outputJSON(out)
	default:
		return outputText(out)
	}
}

func outputJSON(out pathOutput) error {
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputText(out pathOutput) error {
	if !out.Found {
		fmt.Printf("No path found from %q to %q\n", out.From, out.To)
		fmt.Printf("Explored %d nodes in %dms\n", out.Explored, out.DurationMs)
		return nil
	}

	fmt.Printf("Path found (%d hops):\n", out.Hops)
	for i, title := range out.Path {
		if i == 0 {
			fmt.Printf("  %s\n", title)
		} else {
			fmt.Printf("  → %s\n", title)
		}
	}
	fmt.Println()
	fmt.Printf("Explored %s nodes in %dms (%s)\n",
		formatNumber(out.Explored), out.DurationMs, out.Algorithm)

	return nil
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
