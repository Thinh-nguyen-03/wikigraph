// Package cache provides a repository layer for page and link operations.
package cache

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
)

type Cache struct {
	db *database.DB
}

func New(db *database.DB) *Cache {
	return &Cache{db: db}
}

type FetchStatus string

const (
	StatusPending  FetchStatus = "pending"
	StatusSuccess  FetchStatus = "success"
	StatusRedirect FetchStatus = "redirect"
	StatusNotFound FetchStatus = "not_found"
	StatusError    FetchStatus = "error"
)

type Page struct {
	ID          int64
	Title       string
	ContentHash sql.NullString
	FetchStatus FetchStatus
	RedirectTo  sql.NullString
	FetchedAt   sql.NullString
	CreatedAt   string
	UpdatedAt   string
}

type Link struct {
	ID          int64
	SourceID    int64
	TargetTitle string
	CreatedAt   string
}

const pageColumns = "id, title, content_hash, fetch_status, redirect_to, fetched_at, created_at, updated_at"

type scanner interface {
	Scan(dest ...any) error
}

func scanPage(s scanner) (*Page, error) {
	p := &Page{}
	err := s.Scan(&p.ID, &p.Title, &p.ContentHash, &p.FetchStatus, &p.RedirectTo, &p.FetchedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *Cache) GetPage(title string) (*Page, error) {
	row := c.db.QueryRow(`SELECT `+pageColumns+` FROM pages WHERE title = ?`, title)
	p, err := scanPage(row)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying page: %w", err)
	}
	return p, nil
}

func (c *Cache) CreatePage(title string) (*Page, error) {
	result, err := c.db.Exec(`INSERT INTO pages (title) VALUES (?)`, title)
	if err != nil {
		return nil, fmt.Errorf("inserting page: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}
	return c.getPageByID(id)
}

func (c *Cache) GetOrCreatePage(title string) (*Page, error) {
	page, err := c.GetPage(title)
	if err != nil {
		return nil, fmt.Errorf("checking existing page: %w", err)
	}
	if page != nil {
		return page, nil
	}
	return c.CreatePage(title)
}

func (c *Cache) UpdatePageStatus(title string, status FetchStatus, contentHash, redirectTo string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var hashPtr, redirectPtr *string
	if contentHash != "" {
		hashPtr = &contentHash
	}
	if redirectTo != "" {
		redirectPtr = &redirectTo
	}

	_, err := c.db.Exec(`
		UPDATE pages
		SET fetch_status = ?, content_hash = ?, redirect_to = ?, fetched_at = ?, updated_at = ?
		WHERE title = ?
	`, status, hashPtr, redirectPtr, now, now, title)

	if err != nil {
		return fmt.Errorf("updating page status: %w", err)
	}
	return nil
}

func (c *Cache) GetPendingPages(limit int) ([]*Page, error) {
	rows, err := c.db.Query(`
		SELECT `+pageColumns+`
		FROM pages WHERE fetch_status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending pages: %w", err)
	}
	defer rows.Close()

	return scanPages(rows)
}

func (c *Cache) GetStalePages(olderThan time.Duration, limit int) ([]*Page, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339)

	rows, err := c.db.Query(`
		SELECT `+pageColumns+`
		FROM pages
		WHERE fetch_status = 'success' AND fetched_at < ?
		ORDER BY fetched_at ASC
		LIMIT ?
	`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("querying stale pages: %w", err)
	}
	defer rows.Close()

	return scanPages(rows)
}

func (c *Cache) AddLinks(sourceID int64, links []Link) error {
	if len(links) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	const batchSize = 500
	for i := 0; i < len(links); i += batchSize {
		end := i + batchSize
		if end > len(links) {
			end = len(links)
		}
		batch := links[i:end]

		var placeholders []string
		var args []interface{}
		for _, link := range batch {
			placeholders = append(placeholders, "(?, ?)")
			args = append(args, sourceID, link.TargetTitle)
		}

		query := fmt.Sprintf(`
			INSERT OR IGNORE INTO links (source_id, target_title)
			VALUES %s
		`, strings.Join(placeholders, ", "))

		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("inserting links batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (c *Cache) GetOutgoingLinks(sourceID int64) ([]string, error) {
	rows, err := c.db.Query(`SELECT target_title FROM links WHERE source_id = ?`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("querying outgoing links: %w", err)
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("scanning link: %w", err)
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}

func (c *Cache) GetIncomingLinks(targetTitle string) ([]int64, error) {
	rows, err := c.db.Query(`SELECT source_id FROM links WHERE target_title = ?`, targetTitle)
	if err != nil {
		return nil, fmt.Errorf("querying incoming links: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning source_id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (c *Cache) DeleteLinksFromPage(sourceID int64) error {
	_, err := c.db.Exec(`DELETE FROM links WHERE source_id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("deleting links: %w", err)
	}
	return nil
}

func (c *Cache) ReplaceLinks(sourceID int64, links []Link) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM links WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("deleting old links: %w", err)
	}

	if len(links) == 0 {
		return tx.Commit()
	}

	const batchSize = 500
	for i := 0; i < len(links); i += batchSize {
		end := i + batchSize
		if end > len(links) {
			end = len(links)
		}
		batch := links[i:end]

		var placeholders []string
		var args []interface{}
		for _, link := range batch {
			placeholders = append(placeholders, "(?, ?)")
			args = append(args, sourceID, link.TargetTitle)
		}

		query := fmt.Sprintf(`
			INSERT OR IGNORE INTO links (source_id, target_title)
			VALUES %s
		`, strings.Join(placeholders, ", "))

		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("inserting links batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (c *Cache) EnsureTargetPagesExist(titles []string) error {
	if len(titles) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	const batchSize = 500
	for i := 0; i < len(titles); i += batchSize {
		end := i + batchSize
		if end > len(titles) {
			end = len(titles)
		}
		batch := titles[i:end]

		var placeholders []string
		var args []interface{}
		for _, title := range batch {
			placeholders = append(placeholders, "(?)")
			args = append(args, title)
		}

		query := fmt.Sprintf(`
			INSERT OR IGNORE INTO pages (title)
			VALUES %s
		`, strings.Join(placeholders, ", "))

		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("inserting pages batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (c *Cache) getPageByID(id int64) (*Page, error) {
	row := c.db.QueryRow(`SELECT `+pageColumns+` FROM pages WHERE id = ?`, id)
	p, err := scanPage(row)
	if err != nil {
		return nil, fmt.Errorf("querying page by id: %w", err)
	}
	return p, nil
}

func scanPages(rows *sql.Rows) ([]*Page, error) {
	var pages []*Page
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning page: %w", err)
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

type GraphData struct {
	Nodes []string     // Isolated pages with no outgoing links
	Edges [][2]string  // [source, target] pairs
}

func (c *Cache) GetGraphData() (*GraphData, error) {
	data := &GraphData{}

	rows, err := c.db.Query(`
		SELECT p.title, l.target_title
		FROM links l
		INDEXED BY idx_links_source_target_covering
		JOIN pages p ON p.id = l.source_id
		WHERE p.fetch_status = 'success'
	`)
	if err != nil {
		return nil, fmt.Errorf("querying edges: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var source, target string
		if err := rows.Scan(&source, &target); err != nil {
			return nil, fmt.Errorf("scanning edge: %w", err)
		}
		data.Edges = append(data.Edges, [2]string{source, target})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating edges: %w", err)
	}

	rows, err = c.db.Query(`
		SELECT p.title FROM pages p
		LEFT JOIN links l ON l.source_id = p.id
		WHERE p.fetch_status = 'success' AND l.id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("querying isolated nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("scanning isolated node: %w", err)
		}
		data.Nodes = append(data.Nodes, title)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating isolated nodes: %w", err)
	}

	return data, nil
}
