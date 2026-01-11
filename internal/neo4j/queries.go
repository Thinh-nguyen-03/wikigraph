package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// PageNode represents a page node in the graph
type PageNode struct {
	Title string
}

// PathResult represents a path between two pages
type PathResult struct {
	Titles []string
	Length int
}

// CreateNode creates a single page node
func (c *Client) CreateNode(ctx context.Context, title string) error {
	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MERGE (p:Page {title: $title})
		`
		params := map[string]interface{}{
			"title": title,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// CreateNodesBatch creates multiple page nodes in a single transaction using UNWIND
func (c *Client) CreateNodesBatch(ctx context.Context, titles []string) error {
	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			UNWIND $titles AS title
			MERGE (p:Page {title: title})
		`
		params := map[string]interface{}{
			"titles": titles,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// EdgeInput represents a link between two pages
type EdgeInput struct {
	SourceTitle string
	TargetTitle string
}

// CreateEdge creates a single LINKS_TO relationship
func (c *Client) CreateEdge(ctx context.Context, sourceTitle, targetTitle string) error {
	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (source:Page {title: $sourceTitle})
			MATCH (target:Page {title: $targetTitle})
			MERGE (source)-[:LINKS_TO]->(target)
		`
		params := map[string]interface{}{
			"sourceTitle": sourceTitle,
			"targetTitle": targetTitle,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// CreateEdgesBatch creates multiple LINKS_TO relationships in a single transaction
func (c *Client) CreateEdgesBatch(ctx context.Context, edges []EdgeInput) error {
	// Convert EdgeInput structs to maps for Neo4j driver
	edgeMaps := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		edgeMaps[i] = map[string]interface{}{
			"sourceTitle": edge.SourceTitle,
			"targetTitle": edge.TargetTitle,
		}
	}

	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			UNWIND $edges AS edge
			MATCH (source:Page {title: edge.sourceTitle})
			MATCH (target:Page {title: edge.targetTitle})
			MERGE (source)-[:LINKS_TO]->(target)
		`
		params := map[string]interface{}{
			"edges": edgeMaps,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// FindShortestPath finds the shortest path between two pages
// maxDepth limits the search depth (default: 6 for Wikipedia)
func (c *Client) FindShortestPath(ctx context.Context, fromTitle, toTitle string, maxDepth int) (*PathResult, error) {
	if maxDepth == 0 {
		maxDepth = 6
	}

	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := fmt.Sprintf(`
			MATCH (start:Page {title: $fromTitle}), (end:Page {title: $toTitle})
			MATCH path = shortestPath((start)-[:LINKS_TO*1..%d]-(end))
			RETURN [node in nodes(path) | node.title] AS titles, length(path) AS length
		`, maxDepth)

		params := map[string]interface{}{
			"fromTitle": fromTitle,
			"toTitle":   toTitle,
		}

		queryResult, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		if queryResult.Next(ctx) {
			record := queryResult.Record()
			titles, _ := record.Get("titles")
			length, _ := record.Get("length")

			// Convert titles to string slice
			titleInterfaces := titles.([]interface{})
			titleStrings := make([]string, len(titleInterfaces))
			for i, t := range titleInterfaces {
				titleStrings[i] = t.(string)
			}

			return &PathResult{
				Titles: titleStrings,
				Length: int(length.(int64)),
			}, nil
		}

		// No path found
		return nil, nil
	})

	if err != nil {
		return nil, fmt.Errorf("shortest path query failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	return result.(*PathResult), nil
}

// GetOutLinks returns all outgoing links from a page
func (c *Client) GetOutLinks(ctx context.Context, title string, limit int) ([]string, error) {
	if limit == 0 {
		limit = 100
	}

	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := fmt.Sprintf(`
			MATCH (p:Page {title: $title})-[:LINKS_TO]->(target:Page)
			RETURN target.title AS title
			LIMIT %d
		`, limit)

		params := map[string]interface{}{
			"title": title,
		}

		queryResult, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var titles []string
		for queryResult.Next(ctx) {
			record := queryResult.Record()
			title, _ := record.Get("title")
			titles = append(titles, title.(string))
		}

		return titles, queryResult.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("get out links query failed: %w", err)
	}

	return result.([]string), nil
}

// GetInLinks returns all incoming links to a page
func (c *Client) GetInLinks(ctx context.Context, title string, limit int) ([]string, error) {
	if limit == 0 {
		limit = 100
	}

	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := fmt.Sprintf(`
			MATCH (source:Page)-[:LINKS_TO]->(p:Page {title: $title})
			RETURN source.title AS title
			LIMIT %d
		`, limit)

		params := map[string]interface{}{
			"title": title,
		}

		queryResult, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var titles []string
		for queryResult.Next(ctx) {
			record := queryResult.Record()
			title, _ := record.Get("title")
			titles = append(titles, title.(string))
		}

		return titles, queryResult.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("get in links query failed: %w", err)
	}

	return result.([]string), nil
}

// GetConnections returns all pages within N hops of the given page
func (c *Client) GetConnections(ctx context.Context, title string, depth int, limit int) ([]string, error) {
	if depth == 0 {
		depth = 2
	}
	if limit == 0 {
		limit = 100
	}

	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := fmt.Sprintf(`
			MATCH (start:Page {title: $title})-[:LINKS_TO*1..%d]-(neighbor:Page)
			RETURN DISTINCT neighbor.title AS title
			LIMIT %d
		`, depth, limit)

		params := map[string]interface{}{
			"title": title,
		}

		queryResult, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var titles []string
		for queryResult.Next(ctx) {
			record := queryResult.Record()
			title, _ := record.Get("title")
			titles = append(titles, title.(string))
		}

		return titles, queryResult.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("get connections query failed: %w", err)
	}

	return result.([]string), nil
}

// PageExists checks if a page exists in the graph
func (c *Client) PageExists(ctx context.Context, title string) (bool, error) {
	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (p:Page {title: $title})
			RETURN count(p) > 0 AS exists
		`
		params := map[string]interface{}{
			"title": title,
		}

		queryResult, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		if queryResult.Next(ctx) {
			record := queryResult.Record()
			exists, _ := record.Get("exists")
			return exists.(bool), nil
		}

		return false, nil
	})

	if err != nil {
		return false, fmt.Errorf("page exists query failed: %w", err)
	}

	return result.(bool), nil
}

// DeleteNode deletes a page node and all its relationships
func (c *Client) DeleteNode(ctx context.Context, title string) error {
	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (p:Page {title: $title})
			DETACH DELETE p
		`
		params := map[string]interface{}{
			"title": title,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// ClearDatabase deletes all nodes and relationships (use with caution!)
func (c *Client) ClearDatabase(ctx context.Context) error {
	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `MATCH (n) DETACH DELETE n`
		_, err := tx.Run(ctx, query, nil)
		return nil, err
	})
	return err
}
