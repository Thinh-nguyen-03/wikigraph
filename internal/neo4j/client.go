package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Client wraps the Neo4j driver and provides connection management
type Client struct {
	driver neo4j.DriverWithContext
	uri    string
	auth   neo4j.AuthToken
}

// Config holds Neo4j connection configuration
type Config struct {
	URI      string
	Username string
	Password string
	// Connection pool settings
	MaxConnectionPoolSize   int
	ConnectionAcquisitionTimeout time.Duration
}

// NewClient creates a new Neo4j client with the given configuration
func NewClient(cfg Config) (*Client, error) {
	// Set defaults
	if cfg.MaxConnectionPoolSize == 0 {
		cfg.MaxConnectionPoolSize = 50
	}
	if cfg.ConnectionAcquisitionTimeout == 0 {
		cfg.ConnectionAcquisitionTimeout = 60 * time.Second
	}

	auth := neo4j.BasicAuth(cfg.Username, cfg.Password, "")

	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		auth,
		func(config *neo4j.Config) {
			config.MaxConnectionPoolSize = cfg.MaxConnectionPoolSize
			config.ConnectionAcquisitionTimeout = cfg.ConnectionAcquisitionTimeout
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	client := &Client{
		driver: driver,
		uri:    cfg.URI,
		auth:   auth,
	}

	return client, nil
}

// VerifyConnectivity checks if the Neo4j database is reachable
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	if err := c.driver.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("Neo4j connectivity check failed: %w", err)
	}
	return nil
}

// Close closes the driver connection pool
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// NewSession creates a new database session
func (c *Client) NewSession(ctx context.Context) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
}

// NewWriteSession creates a new database session with write access
func (c *Client) NewWriteSession(ctx context.Context) neo4j.SessionWithContext {
	return c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
}

// ExecuteRead runs a read transaction
func (c *Client) ExecuteRead(ctx context.Context, work neo4j.ManagedTransactionWork) (interface{}, error) {
	session := c.NewSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, work)
	if err != nil {
		return nil, fmt.Errorf("read transaction failed: %w", err)
	}
	return result, nil
}

// ExecuteWrite runs a write transaction
func (c *Client) ExecuteWrite(ctx context.Context, work neo4j.ManagedTransactionWork) (interface{}, error) {
	session := c.NewWriteSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, work)
	if err != nil {
		return nil, fmt.Errorf("write transaction failed: %w", err)
	}
	return result, nil
}

// GetStats returns basic database statistics
func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	result, err := c.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// Count nodes
		nodeResult, err := tx.Run(ctx, "MATCH (n:Page) RETURN count(n) as count", nil)
		if err != nil {
			return nil, err
		}
		nodeRecord, err := nodeResult.Single(ctx)
		if err != nil {
			return nil, err
		}
		nodeCount, _ := nodeRecord.Get("count")

		// Count edges
		edgeResult, err := tx.Run(ctx, "MATCH ()-[r:LINKS_TO]->() RETURN count(r) as count", nil)
		if err != nil {
			return nil, err
		}
		edgeRecord, err := edgeResult.Single(ctx)
		if err != nil {
			return nil, err
		}
		edgeCount, _ := edgeRecord.Get("count")

		return &Stats{
			NodeCount: nodeCount.(int64),
			EdgeCount: edgeCount.(int64),
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*Stats), nil
}

// Stats holds database statistics
type Stats struct {
	NodeCount int64
	EdgeCount int64
}

// InitializeSchema creates the necessary constraints and indexes
func (c *Client) InitializeSchema(ctx context.Context) error {
	session := c.NewWriteSession(ctx)
	defer session.Close(ctx)

	// Create unique constraint on Page.title
	// This also creates an index automatically
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			CREATE CONSTRAINT page_title_unique IF NOT EXISTS
			FOR (p:Page) REQUIRE p.title IS UNIQUE
		`
		_, err := tx.Run(ctx, query, nil)
		return nil, err
	})

	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}
