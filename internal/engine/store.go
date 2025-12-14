package engine

import (
	"fmt"
	"sync"
)

// Store represents an in-memory key-value store with thread-safe operations
type Store struct {
	data map[string]string
	mu   sync.RWMutex
}

// NewStore creates a new Store instance
func NewStore() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Set stores a key-value pair in the store
func (s *Store) Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data[key] = value
	return nil
}

// Get retrieves a value by key from the store
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	value, exists := s.data[key]
	return value, exists
}

// Delete removes a key-value pair from the store
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	_, exists := s.data[key]
	if exists {
		delete(s.data, key)
	}
	return exists
}

// Exists checks if a key exists in the store
func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	_, exists := s.data[key]
	return exists
}

// Keys returns all keys in the store
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Clear removes all key-value pairs from the store
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data = make(map[string]string)
}

// Size returns the number of key-value pairs in the store
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return len(s.data)
}