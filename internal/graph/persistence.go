package graph

import (
	"encoding/gob"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// CacheVersion is incremented when the cache format changes.
// Caches with different versions are automatically invalidated.
const CacheVersion = 1

// SerializableGraph represents a graph in a format suitable for disk serialization.
type SerializableGraph struct {
	Version   int
	Nodes     map[string]*SerializableNode
	EdgeCount int
	Timestamp time.Time
}

// SerializableNode represents a node in serializable format.
// We store titles instead of pointers since pointers can't be serialized.
type SerializableNode struct {
	Title         string
	OutLinkTitles []string
	InLinkTitles  []string
}

// Save persists the graph to disk using gob encoding.
// Uses atomic write (temp file + rename) to prevent corruption.
func (g *Graph) Save(path string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.saveLocked(path)
}

func (g *Graph) saveLocked(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Convert to serializable format
	sg := &SerializableGraph{
		Version:   CacheVersion,
		Nodes:     make(map[string]*SerializableNode, len(g.nodes)),
		EdgeCount: g.edges,
		Timestamp: time.Now(),
	}

	for title, node := range g.nodes {
		sn := &SerializableNode{
			Title:         title,
			OutLinkTitles: make([]string, len(node.OutLinks)),
			InLinkTitles:  make([]string, len(node.InLinks)),
		}

		for i, link := range node.OutLinks {
			sn.OutLinkTitles[i] = link.Title
		}
		for i, link := range node.InLinks {
			sn.InLinkTitles[i] = link.Title
		}

		sg.Nodes[title] = sn
	}

	// Write to temporary file first for atomic operation
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp cache file: %w", err)
	}

	encoder := gob.NewEncoder(f)
	if err := encoder.Encode(sg); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encoding graph: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing cache file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing cache file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming cache file: %w", err)
	}

	slog.Info("graph saved to disk",
		"path", path,
		"nodes", len(sg.Nodes),
		"edges", sg.EdgeCount,
	)

	return nil
}

// LoadFromCache loads a graph from a disk cache file.
// Returns the graph and its age (time since cache was created).
func LoadFromCache(path string) (*Graph, time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("cache not found: %w", err)
		}
		return nil, 0, fmt.Errorf("opening cache file: %w", err)
	}
	defer f.Close()

	decoder := gob.NewDecoder(f)
	var sg SerializableGraph
	if err := decoder.Decode(&sg); err != nil {
		return nil, 0, fmt.Errorf("decoding cache: %w", err)
	}

	// Version check
	if sg.Version != CacheVersion {
		return nil, 0, fmt.Errorf("cache version mismatch: got %d, want %d", sg.Version, CacheVersion)
	}

	// Reconstruct graph from serialized format
	g := NewWithCapacity(len(sg.Nodes))
	g.edges = sg.EdgeCount

	// First pass: create all nodes
	for title, sn := range sg.Nodes {
		node := &Node{
			Title:    sn.Title,
			OutLinks: make([]*Node, 0, len(sn.OutLinkTitles)),
			InLinks:  make([]*Node, 0, len(sn.InLinkTitles)),
		}
		g.nodes[title] = node
	}

	// Second pass: wire up connections
	for title, sn := range sg.Nodes {
		node := g.nodes[title]

		for _, outTitle := range sn.OutLinkTitles {
			if target := g.nodes[outTitle]; target != nil {
				node.OutLinks = append(node.OutLinks, target)
			}
		}

		for _, inTitle := range sn.InLinkTitles {
			if source := g.nodes[inTitle]; source != nil {
				node.InLinks = append(node.InLinks, source)
			}
		}
	}

	age := time.Since(sg.Timestamp)

	slog.Info("graph loaded from cache",
		"path", path,
		"nodes", len(g.nodes),
		"edges", g.edges,
		"cache_age", age.Round(time.Second),
	)

	return g, age, nil
}

// CacheExists checks if a cache file exists at the given path.
func CacheExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetCacheInfo returns metadata about the cache without loading the full graph.
func GetCacheInfo(path string) (*CacheInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	decoder := gob.NewDecoder(f)
	var sg SerializableGraph
	if err := decoder.Decode(&sg); err != nil {
		return nil, err
	}

	stat, _ := f.Stat()
	size := int64(0)
	if stat != nil {
		size = stat.Size()
	}

	return &CacheInfo{
		Version:   sg.Version,
		NodeCount: len(sg.Nodes),
		EdgeCount: sg.EdgeCount,
		Timestamp: sg.Timestamp,
		Age:       time.Since(sg.Timestamp),
		FileSize:  size,
		Valid:     sg.Version == CacheVersion,
	}, nil
}

// CacheInfo contains metadata about a graph cache file.
type CacheInfo struct {
	Version   int
	NodeCount int
	EdgeCount int
	Timestamp time.Time
	Age       time.Duration
	FileSize  int64
	Valid     bool
}

// DeleteCache removes the cache file if it exists.
func DeleteCache(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
