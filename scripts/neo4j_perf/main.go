package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/neostore"
)

func main() {
	ctx := context.Background()

	client, err := neostore.NewClient(neostore.Config{
		URI:      "bolt://localhost:7687",
		Username: "neo4j",
		Password: "wikigraph",
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close(ctx)

	if err := client.VerifyConnectivity(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	fmt.Println("Connected to Neo4j")

	fmt.Println("\nGetting sample pages...")
	outLinks, err := client.GetOutLinks(ctx, "Python_(programming_language)", 20)
	if err != nil {
		log.Fatalf("Failed to get out links: %v", err)
	}
	if len(outLinks) == 0 {
		log.Fatal("No outgoing links found from Python_(programming_language)")
	}
	fmt.Printf("Found %d outgoing links from Python_(programming_language)\n", len(outLinks))
	fmt.Printf("Sample targets: %v\n", outLinks[:min(5, len(outLinks))])

	type pair struct{ from, to string }
	testPaths := []pair{
		{"Python_(programming_language)", outLinks[0]},
		{"Python_(programming_language)", outLinks[min(5, len(outLinks)-1)]},
		{"Python_(programming_language)", outLinks[min(10, len(outLinks)-1)]},
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Path Query Performance Tests")
	fmt.Println(strings.Repeat("=", 70))

	var totalDuration time.Duration
	successCount := 0

	for i, test := range testPaths {
		fmt.Printf("\nTest %d: %s -> %s\n", i+1, test.from, test.to)

		start := time.Now()
		path, err := client.FindShortestPath(ctx, test.from, test.to, 6)
		duration := time.Since(start)
		totalDuration += duration

		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			continue
		}
		if path == nil {
			fmt.Printf("  No path found (search took %s)\n", duration)
			continue
		}

		successCount++
		fmt.Printf("  Path found: %d hops in %s\n", path.Length, duration)
		fmt.Printf("  Route: %v\n", path.Titles)
		if duration.Milliseconds() > 20 {
			fmt.Printf("  WARNING: Query took >20ms (target: <20ms)\n")
		} else {
			fmt.Println("  Performance target met (<20ms)")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Summary")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Successful queries: %d/%d\n", successCount, len(testPaths))
	if successCount > 0 {
		avg := totalDuration / time.Duration(successCount)
		fmt.Printf("Average query time: %s\n", avg)
		if avg.Milliseconds() <= 20 {
			fmt.Println("All queries met performance target (<20ms)")
		} else {
			fmt.Printf("Average: %dms (target: <20ms)\n", avg.Milliseconds())
		}
	}
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\nDatabase Statistics:")
	stats, err := client.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}
	fmt.Printf("  Nodes: %d\n", stats.NodeCount)
	fmt.Printf("  Edges: %d\n", stats.EdgeCount)
}
