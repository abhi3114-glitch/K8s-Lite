package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
)

// MemoryStore implements Store interface using an in-memory map.
type MemoryStore struct {
	lock     sync.RWMutex
	data     map[string][]byte
	watchers []*memoryWatcher
	filePath string
}

func NewMemoryStore(filePath string) *MemoryStore {
	store := &MemoryStore{
		data:     make(map[string][]byte),
		filePath: filePath,
	}
	if filePath != "" {
		if err := store.load(); err != nil {
			fmt.Printf("Warning: failed to load store from %s: %v\n", filePath, err)
		}
	}
	return store
}

func (s *MemoryStore) load() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.data)
}

func (s *MemoryStore) sync() error {
	if s.filePath == "" {
		return nil
	}

	// Simple dump
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *MemoryStore) Create(ctx context.Context, key string, obj interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.data[key]; exists {
		return ErrConflict
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	s.data[key] = data
	s.notifyWatchers(Added, key, obj)
	return s.sync()
}

func (s *MemoryStore) Update(ctx context.Context, key string, obj interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.data[key]; !exists {
		return ErrNotFound
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	s.data[key] = data
	s.notifyWatchers(Modified, key, obj)
	return s.sync()
}

func (s *MemoryStore) Get(ctx context.Context, key string, objPtr interface{}) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	data, exists := s.data[key]
	if !exists {
		return ErrNotFound
	}

	return json.Unmarshal(data, objPtr)
}

func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exists := s.data[key]; !exists {
		return ErrNotFound
	}

	// For delete, we need the object to notify watchers.
	// In a real generic store, we might only send the key or tombstone.
	// Here we try to unmarshal to a generic map just to send something,
	// or assume the caller knows what was deleted.
	// Ideally we'd read it before deleting.
	var oldObj map[string]interface{}
	json.Unmarshal(s.data[key], &oldObj)

	delete(s.data, key)
	s.notifyWatchers(Deleted, key, oldObj)
	return s.sync()
}

func (s *MemoryStore) List(ctx context.Context, keyPrefix string, listObjPtr interface{}) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// Reflect magic to append to slice
	ptrVal := reflect.ValueOf(listObjPtr)
	if ptrVal.Kind() != reflect.Ptr || ptrVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("listObjPtr must be a pointer to a slice")
	}
	sliceVal := ptrVal.Elem()
	elemType := sliceVal.Type().Elem()

	for k, v := range s.data {
		if strings.HasPrefix(k, keyPrefix) {
			newElem := reflect.New(elemType).Interface()
			if err := json.Unmarshal(v, newElem); err != nil {
				return err
			}
			sliceVal.Set(reflect.Append(sliceVal, reflect.ValueOf(newElem).Elem()))
		}
	}
	return nil
}

func (s *MemoryStore) Watch(ctx context.Context, keyPrefix string) (WatchInterface, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	w := &memoryWatcher{
		resultChan: make(chan Event, 10),
		keyPrefix:  keyPrefix,
		store:      s,
	}
	s.watchers = append(s.watchers, w)
	return w, nil
}

func (s *MemoryStore) notifyWatchers(eventType EventType, key string, obj interface{}) {
	for _, w := range s.watchers {
		if strings.HasPrefix(key, w.keyPrefix) {
			w.resultChan <- Event{Type: eventType, Object: obj}
		}
	}
}

type memoryWatcher struct {
	resultChan chan Event
	keyPrefix  string
	store      *MemoryStore
}

func (w *memoryWatcher) Stop() {
	w.store.lock.Lock()
	defer w.store.lock.Unlock()
	// Remove self from watchers list
	for i, watcher := range w.store.watchers {
		if watcher == w {
			w.store.watchers = append(w.store.watchers[:i], w.store.watchers[i+1:]...)
			break
		}
	}
	close(w.resultChan)
}

func (w *memoryWatcher) ResultChan() <-chan Event {
	return w.resultChan
}



