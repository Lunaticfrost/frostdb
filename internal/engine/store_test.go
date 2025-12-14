package engine

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.Size() != 0 {
		t.Errorf("New store should be empty, got size %d", store.Size())
	}
}

func TestSetAndGet(t *testing.T) {
	store := NewStore()

	err := store.Set("name", "Alice")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, exists := store.Get("name")
	if !exists {
		t.Fatal("Key should exist")
	}
	if value != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", value)
	}
}

func TestSetEmptyKey(t *testing.T) {
	store := NewStore()

	err := store.Set("", "value")
	if err == nil {
		t.Error("Setting empty key should return error")
	}
}

func TestGetNonExistentKey(t *testing.T) {
	store := NewStore()

	_, exists := store.Get("nonexistent")
	if exists {
		t.Error("Non-existent key should not exist")
	}
}

func TestDelete(t *testing.T) {
	store := NewStore()

	store.Set("name", "Bob")
	deleted := store.Delete("name")
	if !deleted {
		t.Error("Delete should return true for existing key")
	}

	_, exists := store.Get("name")
	if exists {
		t.Error("Key should not exist after deletion")
	}

	deletedAgain := store.Delete("name")
	if deletedAgain {
		t.Error("Delete should return false for non-existent key")
	}
}

func TestExists(t *testing.T) {
	store := NewStore()

	if store.Exists("test") {
		t.Error("Key should not exist initially")
	}

	store.Set("test", "value")
	if !store.Exists("test") {
		t.Error("Key should exist after Set")
	}

	store.Delete("test")
	if store.Exists("test") {
		t.Error("Key should not exist after Delete")
	}
}

func TestKeys(t *testing.T) {
	store := NewStore()

	keys := store.Keys()
	if len(keys) != 0 {
		t.Errorf("Empty store should have 0 keys, got %d", len(keys))
	}

	store.Set("key1", "value1")
	store.Set("key2", "value2")
	store.Set("key3", "value3")

	keys = store.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check all keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	expectedKeys := []string{"key1", "key2", "key3"}
	for _, k := range expectedKeys {
		if !keyMap[k] {
			t.Errorf("Expected key '%s' not found", k)
		}
	}
}

func TestClear(t *testing.T) {
	store := NewStore()

	store.Set("key1", "value1")
	store.Set("key2", "value2")

	if store.Size() != 2 {
		t.Errorf("Expected size 2, got %d", store.Size())
	}

	store.Clear()

	if store.Size() != 0 {
		t.Errorf("Expected size 0 after Clear, got %d", store.Size())
	}

	if store.Exists("key1") {
		t.Error("Key should not exist after Clear")
	}
}

func TestSize(t *testing.T) {
	store := NewStore()

	if store.Size() != 0 {
		t.Errorf("Expected size 0, got %d", store.Size())
	}

	store.Set("key1", "value1")
	if store.Size() != 1 {
		t.Errorf("Expected size 1, got %d", store.Size())
	}

	store.Set("key2", "value2")
	if store.Size() != 2 {
		t.Errorf("Expected size 2, got %d", store.Size())
	}

	store.Delete("key1")
	if store.Size() != 1 {
		t.Errorf("Expected size 1 after delete, got %d", store.Size())
	}
}

func TestOverwriteValue(t *testing.T) {
	store := NewStore()

	store.Set("key", "value1")
	value, _ := store.Get("key")
	if value != "value1" {
		t.Errorf("Expected 'value1', got '%s'", value)
	}

	store.Set("key", "value2")
	value, _ = store.Get("key")
	if value != "value2" {
		t.Errorf("Expected 'value2', got '%s'", value)
	}

	if store.Size() != 1 {
		t.Errorf("Expected size 1, got %d", store.Size())
	}
}

// TestConcurrentWrites tests thread-safety with concurrent writes
func TestConcurrentWrites(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup

	numGoroutines := 100
	writesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)
				store.Set(key, value)
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numGoroutines * writesPerGoroutine
	if store.Size() != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, store.Size())
	}
}

// TestConcurrentReads tests thread-safety with concurrent reads
func TestConcurrentReads(t *testing.T) {
	store := NewStore()

	// Populate store
	for i := 0; i < 100; i++ {
		store.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j)
				value, exists := store.Get(key)
				if !exists {
					t.Errorf("Key %s should exist", key)
				}
				expectedValue := fmt.Sprintf("value-%d", j)
				if value != expectedValue {
					t.Errorf("Expected %s, got %s", expectedValue, value)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentReadWrite tests thread-safety with mixed operations
func TestConcurrentReadWrite(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j)
				value := fmt.Sprintf("value-%d-%d", id, j)
				store.Set(key, value)
			}
		}(i)
	}

	// Readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j)
				store.Get(key) // Just read, don't validate (data is changing)
			}
		}()
	}

	wg.Wait()

	// Verify no crashes and store is in valid state
	if store.Size() < 0 || store.Size() > 100 {
		t.Errorf("Store size %d is invalid", store.Size())
	}
}

// Benchmark tests
func BenchmarkSet(b *testing.B) {
	store := NewStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		store.Set(key, value)
	}
}

func BenchmarkGet(b *testing.B) {
	store := NewStore()

	// Populate store
	for i := 0; i < 10000; i++ {
		store.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%10000)
		store.Get(key)
	}
}

func BenchmarkConcurrentSet(b *testing.B) {
	store := NewStore()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			store.Set(key, value)
			i++
		}
	})
}

func BenchmarkConcurrentGet(b *testing.B) {
	store := NewStore()

	// Populate store
	for i := 0; i < 10000; i++ {
		store.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%10000)
			store.Get(key)
			i++
		}
	})
}
