package neostore

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

	// Handle same-page case
	if fromTitle == toTitle {
		return &PathResult{
			Titles: []string{fromTitle},
			Length: 0,
		}, nil
	}

	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := fmt.Sprintf(`
			MATCH (start:Page {title: $fromTitle}), (end:Page {title: $toTitle})
			MATCH path = shortestPath((start)-[:LINKS_TO*1..%d]->(end))
			RETURN [node in nodes(path) | node.title] AS titles, length(path) AS length
			LIMIT 1
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
		query := `
			MATCH (p:Page {title: $title})-[:LINKS_TO]->(target:Page)
			RETURN target.title AS title
			LIMIT $limit
		`

		params := map[string]interface{}{
			"title": title,
			"limit": limit,
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
		query := `
			MATCH (source:Page)-[:LINKS_TO]->(p:Page {title: $title})
			RETURN source.title AS title
			LIMIT $limit
		`

		params := map[string]interface{}{
			"title": title,
			"limit": limit,
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

// NeighborhoodNode represents a node in the neighborhood with hop distance
type NeighborhoodNode struct {
	Title string
	Hops  int
}

// NeighborhoodEdge represents an edge in the neighborhood
type NeighborhoodEdge struct {
	Source string
	Target string
}

// NeighborhoodResult contains the full N-hop neighborhood
type NeighborhoodResult struct {
	Nodes []NeighborhoodNode
	Edges []NeighborhoodEdge
}

// GetNeighborhood returns the N-hop neighborhood with nodes, edges, and hop distances
func (c *Client) GetNeighborhood(ctx context.Context, title string, depth int, maxNodes int) (*NeighborhoodResult, error) {
	if depth == 0 {
		depth = 2
	}
	if maxNodes == 0 {
		maxNodes = 1000
	}

	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// Get nodes with their minimum hop distance (using directed paths in both directions)
		nodesQuery := fmt.Sprintf(`
			MATCH (start:Page {title: $title})
			CALL {
				WITH start
				MATCH path = (start)-[:LINKS_TO*1..%d]->(neighbor:Page)
				WHERE neighbor <> start
				RETURN neighbor, length(path) AS hops
				UNION
				WITH start
				MATCH path = (start)<-[:LINKS_TO*1..%d]-(neighbor:Page)
				WHERE neighbor <> start
				RETURN neighbor, length(path) AS hops
			}
			WITH neighbor, min(hops) AS minHops
			RETURN neighbor.title AS title, minHops AS hops
			ORDER BY minHops, title
			LIMIT %d
		`, depth, depth, maxNodes)

		params := map[string]interface{}{
			"title": title,
		}

		nodesResult, err := tx.Run(ctx, nodesQuery, params)
		if err != nil {
			return nil, err
		}

		nodeSet := make(map[string]int)
		nodeSet[title] = 0 // Include center node

		for nodesResult.Next(ctx) {
			record := nodesResult.Record()
			nodeTitle, _ := record.Get("title")
			hops, _ := record.Get("hops")
			nodeSet[nodeTitle.(string)] = int(hops.(int64))
		}
		if err := nodesResult.Err(); err != nil {
			return nil, err
		}

		// Build node list
		nodes := make([]NeighborhoodNode, 0, len(nodeSet))
		for t, h := range nodeSet {
			nodes = append(nodes, NeighborhoodNode{Title: t, Hops: h})
		}

		// Get edges between nodes in the neighborhood
		// Collect node titles for filtering
		nodeTitles := make([]string, 0, len(nodeSet))
		for t := range nodeSet {
			nodeTitles = append(nodeTitles, t)
		}

		edgesQuery := `
			MATCH (n1:Page)-[:LINKS_TO]->(n2:Page)
			WHERE n1.title IN $nodes AND n2.title IN $nodes
			RETURN DISTINCT n1.title AS source, n2.title AS target
		`

		edgesParams := map[string]interface{}{
			"nodes": nodeTitles,
		}

		edgesResult, err := tx.Run(ctx, edgesQuery, edgesParams)
		if err != nil {
			return nil, err
		}

		var edges []NeighborhoodEdge
		for edgesResult.Next(ctx) {
			record := edgesResult.Record()
			source, _ := record.Get("source")
			target, _ := record.Get("target")
			edges = append(edges, NeighborhoodEdge{
				Source: source.(string),
				Target: target.(string),
			})
		}
		if err := edgesResult.Err(); err != nil {
			return nil, err
		}

		return &NeighborhoodResult{
			Nodes: nodes,
			Edges: edges,
		}, nil
	})

	if err != nil {
		return nil, fmt.Errorf("get neighborhood query failed: %w", err)
	}

	return result.(*NeighborhoodResult), nil
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
		// Use directed search for better performance
		query := fmt.Sprintf(`
			MATCH (start:Page {title: $title})-[:LINKS_TO*1..%d]->(neighbor:Page)
			RETURN DISTINCT neighbor.title AS title
			LIMIT $limit
		`, depth)

		params := map[string]interface{}{
			"title": title,
			"limit": limit,
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

// CreateIndexes creates necessary indexes for query performance
// Run this once after database setup
func (c *Client) CreateIndexes(ctx context.Context) error {
	_, err := c.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// Create index on Page.title for faster lookups
		query := `CREATE INDEX page_title_index IF NOT EXISTS FOR (p:Page) ON (p.title)`
		_, err := tx.Run(ctx, query, nil)
		return nil, err
	})
	return err
}

// FindShortestPathBidirectional finds the shortest path between two pages.
// True application-level bidirectional BFS requires APOC (apoc.algo.shortestPath).
// Without APOC, this delegates to FindShortestPath which uses Neo4j's native
// shortestPath() — an optimized internal BFS that is already efficient on large graphs.
func (c *Client) FindShortestPathBidirectional(ctx context.Context, fromTitle, toTitle string, maxDepth int) (*PathResult, error) {
	return c.FindShortestPath(ctx, fromTitle, toTitle, maxDepth)
}
