package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetSetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	type data struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	err := c.Set("ns", "key1", data{Name: "alpha", Count: 42})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got data
	ok, err := c.Get("ns", "key1", &got)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Name != "alpha" || got.Count != 42 {
		t.Errorf("got %+v, want {Name:alpha Count:42}", got)
	}
}

func TestGetMiss(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	var got struct{ X int }
	ok, err := c.Get("ns", "missing", &got)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("expected cache miss")
	}
}

func TestTTLExpiry(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, WithTTL(time.Minute))

	err := c.Set("ns", "key1", "hello")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got string
	ok, _ := c.Get("ns", "key1", &got)
	if !ok {
		t.Fatal("expected cache hit before TTL")
	}

	path := c.path("ns", "key1")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache entry: %v", err)
	}
	var cached entry
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("decode cache entry: %v", err)
	}
	cached.FetchedAt = time.Now().Add(-2 * time.Minute).UnixNano()
	data, err = json.Marshal(cached)
	if err != nil {
		t.Fatalf("encode expired cache entry: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write expired cache entry: %v", err)
	}

	ok, _ = c.Get("ns", "key1", &got)
	if ok {
		t.Fatal("expected cache miss after TTL")
	}
}

func TestDisabledCache(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, WithEnabled(false))

	err := c.Set("ns", "key1", "value")
	if err != nil {
		t.Fatalf("Set on disabled cache: %v", err)
	}

	var got string
	ok, _ := c.Get("ns", "key1", &got)
	if ok {
		t.Error("disabled cache should not return hits")
	}

	if c.Enabled() {
		t.Error("Enabled() should return false")
	}
}

func TestDeferredWritesRemainInMemoryUntilFlush(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	c := New(dir, WithDeferredWrites(true))
	if err := c.Set("jira", "PROJ-1", "value"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("deferred Set changed filesystem before Flush: %v", err)
	}
	var got string
	if ok, err := c.Get("jira", "PROJ-1", &got); err != nil || !ok || got != "value" {
		t.Fatalf("pending cache read = %q, hit=%v, err=%v", got, ok, err)
	}
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jira", "PROJ-1.json")); err != nil {
		t.Fatalf("flushed entry missing: %v", err)
	}
}

func TestDisabledInvalidate(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, WithEnabled(false))

	if err := c.Invalidate("ns", "key1"); err != nil {
		t.Errorf("Invalidate on disabled cache should not error: %v", err)
	}
}

func TestDisabledInvalidateNamespace(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, WithEnabled(false))

	if err := c.InvalidateNamespace("ns"); err != nil {
		t.Errorf("InvalidateNamespace on disabled cache should not error: %v", err)
	}
}

func TestInvalidate(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns", "key1", "v1")
	c.Set("ns", "key2", "v2")

	err := c.Invalidate("ns", "key1")
	if err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	var got string
	ok, _ := c.Get("ns", "key1", &got)
	if ok {
		t.Error("key1 should be invalidated")
	}

	ok, _ = c.Get("ns", "key2", &got)
	if !ok {
		t.Error("key2 should still exist")
	}
}

func TestInvalidateNamespace(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns1", "key1", "v1")
	c.Set("ns2", "key2", "v2")

	err := c.InvalidateNamespace("ns1")
	if err != nil {
		t.Fatalf("InvalidateNamespace: %v", err)
	}

	var got string
	ok, _ := c.Get("ns1", "key1", &got)
	if ok {
		t.Error("ns1/key1 should be invalidated")
	}

	ok, _ = c.Get("ns2", "key2", &got)
	if !ok {
		t.Error("ns2/key2 should still exist")
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns1", "key1", "v1")
	c.Set("ns2", "key2", "v2")

	err := c.Clear()
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("cache root directory should be removed after Clear")
	}
}

func TestClearThenReuse(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns", "key1", "v1")
	c.Clear()

	err := c.Set("ns", "key2", "v2")
	if err != nil {
		t.Fatalf("Set after Clear: %v", err)
	}

	var got string
	ok, _ := c.Get("ns", "key2", &got)
	if !ok || got != "v2" {
		t.Error("cache should work after Clear")
	}
}

func TestSafeKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PROJ-123", "PROJ-123"},
		{"simple_key", "simple_key"},
		{"a/b\\c:d", "a_0x2f_b_0x5c_c_0x3a_d"},
		{"spaces and such", "spaces_0x20_and_0x20_such"},
		{"UPPER-lower_123", "UPPER-lower_123"},
		{"", "_empty"},
		{"!!!", "_0x21__0x21__0x21_"},
		{"///", "_0x2f__0x2f__0x2f_"},
	}

	for _, tt := range tests {
		got := safeKey(tt.input)
		if got != tt.want {
			t.Errorf("safeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSafeKeyCollisionResistance(t *testing.T) {
	keys := []string{"PROJ-123", "PROJ_123"}
	seen := make(map[string]string)
	for _, k := range keys {
		safe := safeKey(k)
		if prev, exists := seen[safe]; exists {
			t.Errorf("safeKey collision: %q and %q both map to %q", prev, k, safe)
		}
		seen[safe] = k
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	p := c.Path()
	if p != dir {
		t.Errorf("Path() = %q, want %q", p, dir)
	}
}

func TestFileStructure(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("jira", "PROJ-123", "value")

	expected := filepath.Join(dir, "jira", "PROJ-123.json")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expected)
	}
}

func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			c.Set("ns", "key", i)
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			var got int
			c.Get("ns", "key", &got)
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}

func TestInvalidateNonexistentKey(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	err := c.Invalidate("ns", "nope")
	if err != nil {
		t.Errorf("invalidating nonexistent key should not error: %v", err)
	}
}

func TestCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	path := filepath.Join(dir, "ns", "bad.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("{{not json}}"), 0644)

	var got string
	ok, err := c.Get("ns", "bad", &got)
	if err != nil {
		t.Errorf("corrupt JSON should not return error: %v", err)
	}
	if ok {
		t.Error("corrupt JSON should be treated as cache miss")
	}
}

func TestCorruptValueField(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	path := filepath.Join(dir, "ns", "badval.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	badEntry := entry{Value: []byte("{{not json}}"), FetchedAt: time.Now().UnixNano()}
	data, _ := json.Marshal(badEntry)
	os.WriteFile(path, data, 0644)

	var got string
	ok, _ := c.Get("ns", "badval", &got)
	if ok {
		t.Error("corrupt value field should be treated as cache miss")
	}
}

func TestNamespaceIsolation(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns1", "shared", "from-ns1")
	c.Set("ns2", "shared", "from-ns2")

	var got1, got2 string
	ok1, _ := c.Get("ns1", "shared", &got1)
	ok2, _ := c.Get("ns2", "shared", &got2)

	if !ok1 || got1 != "from-ns1" {
		t.Errorf("ns1/shared = %q, want %q", got1, "from-ns1")
	}
	if !ok2 || got2 != "from-ns2" {
		t.Errorf("ns2/shared = %q, want %q", got2, "from-ns2")
	}
}

func TestOverwriteKey(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns", "key", "v1")
	c.Set("ns", "key", "v2")

	var got string
	ok, _ := c.Get("ns", "key", &got)
	if !ok || got != "v2" {
		t.Errorf("overwritten key = %q, want %q", got, "v2")
	}
}

func TestTypeMismatch(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns", "key", "string-value")

	var got int
	ok, _ := c.Get("ns", "key", &got)
	if ok {
		t.Error("type mismatch should be treated as cache miss")
	}
}

func TestDefaultTTL(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	if c.ttl != 24*time.Hour {
		t.Errorf("default TTL = %v, want 24h", c.ttl)
	}
}

func TestWithTTL(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, WithTTL(1*time.Hour))

	if c.ttl != 1*time.Hour {
		t.Errorf("TTL = %v, want 1h", c.ttl)
	}
}

func TestSetCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	err := c.Set("deep", "nested", "value")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	expected := filepath.Join(dir, "deep", "nested.json")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Error("expected nested directory and file to be created")
	}
}

func TestAtomicWriteNoTmpFile(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	c.Set("ns", "key", "value")

	tmpPath := filepath.Join(dir, "ns", "key.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be cleaned up after atomic rename")
	}
}

func TestTTLPersistedAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	c1 := New(dir)

	c1.Set("ns", "key1", "value1")

	c2 := New(dir)

	var got string
	ok, _ := c2.Get("ns", "key1", &got)
	if !ok || got != "value1" {
		t.Error("cache entry should survive across Cache instances")
	}
}

func TestSliceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	val := []string{"alpha", "beta", "gamma"}
	c.Set("ns", "list", val)

	var got []string
	ok, _ := c.Get("ns", "list", &got)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 3 || got[0] != "alpha" || got[2] != "gamma" {
		t.Errorf("got %v, want %v", got, val)
	}
}

func TestMapRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	val := map[string]int{"x": 1, "y": 2}
	c.Set("ns", "map", val)

	var got map[string]int
	ok, _ := c.Get("ns", "map", &got)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got["x"] != 1 || got["y"] != 2 {
		t.Errorf("got %v, want %v", got, val)
	}
}

func TestConcurrentReadWriteDifferentKeys(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	done := make(chan struct{})
	const n = 50

	go func() {
		for i := 0; i < n; i++ {
			c.Set("ns", fmt.Sprintf("key-%d", i), i)
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < n; i++ {
			var got int
			c.Get("ns", fmt.Sprintf("key-%d", i), &got)
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}
