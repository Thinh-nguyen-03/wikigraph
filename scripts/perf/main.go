package main

import (
	"context"
	"fmt"
	"log"
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

	fmt.Println("\nDatabase Statistics:")
	stats, err := client.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}
	fmt.Printf("  Nodes: %d\n", stats.NodeCount)
	fmt.Printf("  Edges: %d\n", stats.EdgeCount)

	testPages := []string{"Music", "Internet", "Python", "Computer"}

	fmt.Println("\nFinding test pages...")
	var validPages []string
	for _, page := range testPages {
		exists, err := client.PageExists(ctx, page)
		if err != nil {
			fmt.Printf("  Error checking %s: %v\n", page, err)
			continue
		}
		if exists {
			fmt.Printf("  Found: %s\n", page)
			validPages = append(validPages, page)
		}
	}

	if len(validPages) < 2 {
		log.Fatal("Not enough test pages found")
	}

	type pair struct{ from, to string }
	testPairs := []pair{{validPages[0], validPages[1]}}
	if len(validPages) >= 3 {
		testPairs = append(testPairs, pair{validPages[1], validPages[2]})
	}
	if len(validPages) >= 4 {
		testPairs = append(testPairs, pair{validPages[0], validPages[3]})
	}

	fmt.Println("\n" + "=============================================================")
	fmt.Println("Path Query Performance Tests")
	fmt.Println("=============================================================")

	var totalDuration time.Duration
	successCount := 0

	for i, test := range testPairs {
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
		if len(path.Titles) <= 10 {
			fmt.Printf("  Route: %v\n", path.Titles)
		} else {
			fmt.Printf("  Route: %s ... %s (%d nodes)\n", path.Titles[0], path.Titles[len(path.Titles)-1], len(path.Titles))
		}
		if duration.Milliseconds() > 20 {
			fmt.Printf("  WARNING: Query took %dms (target: <20ms)\n", duration.Milliseconds())
		} else {
			fmt.Println("  Performance target met (<20ms)")
		}
	}

	fmt.Println("\n" + "=============================================================")
	fmt.Println("Summary")
	fmt.Println("=============================================================")
	fmt.Printf("Successful queries: %d/%d\n", successCount, len(testPairs))
	if successCount > 0 {
		avg := totalDuration / time.Duration(successCount)
		fmt.Printf("Average query time: %s\n", avg)
		if avg.Milliseconds() <= 20 {
			fmt.Println("All queries met performance target (<20ms)")
		} else {
			fmt.Printf("Average: %dms (target: <20ms)\n", avg.Milliseconds())
		}
	}
	fmt.Println("=============================================================")

	if len(validPages) > 0 {
		fmt.Printf("\nTesting GetOutLinks for '%s'...\n", validPages[0])
		start := time.Now()
		links, err := client.GetOutLinks(ctx, validPages[0], 20)
		duration := time.Since(start)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			fmt.Printf("  Found %d links in %s\n", len(links), duration)
			if len(links) > 0 {
				fmt.Printf("  Sample: %v\n", links[:min(5, len(links))])
			}
		}
	}
}
