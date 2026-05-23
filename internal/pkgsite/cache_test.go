package pkgsite

import (
	"testing"
	"time"
)

func TestCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := newCache(time.Minute, 2, 0) // bounded to 2 entries
	c.set("a", []byte("1"))
	c.set("b", []byte("2"))

	// Touch "a" so "b" becomes the least-recently-used entry.
	if _, ok := c.get("a"); !ok {
		t.Fatal("a should be present")
	}
	c.set("c", []byte("3")) // over the entry bound: evicts LRU, which is "b"

	if _, ok := c.get("b"); ok {
		t.Error("b should have been evicted as least-recently-used")
	}
	if _, ok := c.get("a"); !ok {
		t.Error("a should still be present after being promoted")
	}
	if _, ok := c.get("c"); !ok {
		t.Error("c should be present")
	}
}

func TestCacheEvictsByBytes(t *testing.T) {
	c := newCache(time.Minute, 0, 10) // bounded to 10 bytes
	c.set("a", make([]byte, 6))
	c.set("b", make([]byte, 6)) // total 12 > 10: evicts "a"

	if _, ok := c.get("a"); ok {
		t.Error("a should have been evicted to satisfy the byte bound")
	}
	if _, ok := c.get("b"); !ok {
		t.Error("b should be present")
	}
}

func TestCacheSkipsOversizedEntry(t *testing.T) {
	c := newCache(time.Minute, 0, 10)
	c.set("big", make([]byte, 11)) // alone exceeds the whole budget

	if _, ok := c.get("big"); ok {
		t.Error("an entry larger than the byte budget should not be cached")
	}
	if c.curBytes != 0 {
		t.Errorf("curBytes = %d, want 0", c.curBytes)
	}
}

func TestCacheUpdateAdjustsByteTotal(t *testing.T) {
	c := newCache(time.Minute, 0, 100)
	c.set("a", make([]byte, 10))
	c.set("a", make([]byte, 30)) // same key, larger body

	if c.curBytes != 30 {
		t.Errorf("curBytes = %d, want 30 (replaced, not summed)", c.curBytes)
	}
	if got, _ := c.get("a"); len(got) != 30 {
		t.Errorf("len(body) = %d, want 30", len(got))
	}
}

func TestCacheExpiresEntries(t *testing.T) {
	c := newCache(time.Nanosecond, 0, 1<<20)
	c.set("a", []byte("1"))
	time.Sleep(time.Millisecond)

	if _, ok := c.get("a"); ok {
		t.Error("expired entry should not be returned")
	}
	if c.curBytes != 0 {
		t.Errorf("curBytes = %d, want 0 after lazy expiry", c.curBytes)
	}
}
