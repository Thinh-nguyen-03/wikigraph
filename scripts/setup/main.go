package main

import (
	"context"
	"fmt"
	"log"

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

	fmt.Println("Creating indexes...")
	if err := client.CreateIndexes(ctx); err != nil {
		log.Fatalf("Failed to create indexes: %v", err)
	}

	fmt.Println("Indexes created successfully!")
	fmt.Println("\nTo verify indexes, run in cypher-shell:")
	fmt.Println("  SHOW INDEXES")
}
