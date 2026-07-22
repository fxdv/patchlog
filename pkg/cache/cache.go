// Package cache provides a file-based JSON cache with TTL for Jira issues and Confluence page lookups.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fxdv/patchlog/internal/atomicfile"
)

const fallbackKey = "_empty"

func hexEscape(b byte) string {
	return fmt.Sprintf("_0x%02x_", b)
}

type entry struct {
	Value     []byte `json:"value"`
	FetchedAt int64  `json:"fetched_at"`
}

type Cache struct {
	root        string
	ttl         time.Duration
	mu          sync.RWMutex
	enabled     bool
	deferWrites bool
	pending     map[string][]byte
}

type Option func(*Cache)

func WithTTL(ttl time.Duration) Option {
	return func(c *Cache) { c.ttl = ttl }
}

func WithEnabled(enabled bool) Option {
	return func(c *Cache) { c.enabled = enabled }
}

// WithDeferredWrites keeps new entries in memory until Flush. Reads can see
// pending entries, allowing planning to remain filesystem-side-effect-free.
func WithDeferredWrites(deferWrites bool) Option {
	return func(c *Cache) { c.deferWrites = deferWrites }
}

func New(root string, opts ...Option) *Cache {
	c := &Cache{
		root:    root,
		ttl:     24 * time.Hour,
		enabled: true,
		pending: make(map[string][]byte),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Cache) Enabled() bool {
	return c.enabled
}

func (c *Cache) Get(namespace, key string, dest any) (bool, error) {
	if !c.enabled {
		return false, nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	path := c.path(namespace, key)
	if data, ok := c.pending[path]; ok {
		return decodeEntry(data, c.ttl, dest)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return decodeEntry(data, c.ttl, dest)
}

func (c *Cache) Set(namespace, key string, value any) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	e := entry{
		Value:     raw,
		FetchedAt: time.Now().UnixNano(),
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("cache marshal entry: %w", err)
	}

	path := c.path(namespace, key)
	if c.deferWrites {
		c.pending[path] = append([]byte(nil), data...)
		return nil
	}
	if err := atomicfile.Write(path, data, 0644); err != nil {
		return fmt.Errorf("cache write: %w", err)
	}

	return nil
}

// Flush persists deferred entries. It is the explicit cache mutation boundary.
func (c *Cache) Flush() error {
	if !c.enabled {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	paths := make([]string, 0, len(c.pending))
	for path := range c.pending {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := atomicfile.Write(path, c.pending[path], 0644); err != nil {
			return fmt.Errorf("cache flush %s: %w", path, err)
		}
		delete(c.pending, path)
	}
	return nil
}

func (c *Cache) Invalidate(namespace, key string) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.path(namespace, key)
	delete(c.pending, path)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (c *Cache) InvalidateNamespace(namespace string) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	dir := filepath.Join(c.root, namespace)
	prefix := dir + string(filepath.Separator)
	for path := range c.pending {
		if strings.HasPrefix(path, prefix) {
			delete(c.pending, path)
		}
	}
	os.RemoveAll(dir)
	return nil
}

func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.pending)
	return os.RemoveAll(c.root)
}

func decodeEntry(data []byte, ttl time.Duration, dest any) (bool, error) {
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return false, nil
	}
	if time.Since(time.Unix(0, e.FetchedAt)) > ttl {
		return false, nil
	}
	if err := json.Unmarshal(e.Value, dest); err != nil {
		return false, nil
	}
	return true, nil
}

func (c *Cache) Path() string {
	return c.root
}

func (c *Cache) path(namespace, key string) string {
	safe := safeKey(key)
	return filepath.Join(c.root, namespace, safe+".json")
}

func safeKey(key string) string {
	s := make([]byte, 0, len(key))
	for i := 0; i < len(key); i++ {
		b := key[i]
		switch {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
			s = append(s, b)
		case b == '-', b == '_':
			s = append(s, b)
		default:
			s = append(s, hexEscape(b)...)
		}
	}
	if len(s) == 0 {
		return fallbackKey
	}
	return string(s)
}
