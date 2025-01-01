package storage

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("resource conflict")
)

// ListOptions contains options for listing resources
type ListOptions struct {
	LabelSelector map[string]string
	FieldSelector map[string]string
}

// WatchInterface defines the interface for watching resources
type WatchInterface interface {
	Stop()
	// ResultChan returns a channel for events
	ResultChan() <-chan Event
}

// EventType defines the possible types of events.
type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Error    EventType = "ERROR"
)

// Event represents a single event to a watched resource.
type Event struct {
	Type   EventType
	Object interface{}
}

// Store is the interface that all persistence backends must implement
type Store interface {
	// Create adds a new object to the store. Fails if it already exists.
	Create(ctx context.Context, key string, obj interface{}) error

	// Update updates an existing object. Fails if it doesn't exist.
	Update(ctx context.Context, key string, obj interface{}) error

	// Get retrieves an object by key.
	Get(ctx context.Context, key string, objPtr interface{}) error

	// Delete removes an object by key.
	Delete(ctx context.Context, key string) error

	// List retrieves a list of objects matching the prefix key.
	// listObjPtr should be a pointer to a slice of objects.
	List(ctx context.Context, keyPrefix string, listObjPtr interface{}) error

	// Watch returns a channel that receives events for changes to objects matching the key.
	Watch(ctx context.Context, key string) (WatchInterface, error)
}



