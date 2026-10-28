package apalache

import (
	"fmt"
	"sync"
)

// MessageStorage associates diagnostic text with arena/object identifiers.
type MessageStorage interface {
	AddMessage(id int, message string)
	FindMessage(id int) (string, error)
}

type MapMessageStorage struct {
	mu       sync.RWMutex
	messages map[int]string
}

func NewMessageStorage() *MapMessageStorage {
	return &MapMessageStorage{messages: make(map[int]string)}
}

func (storage *MapMessageStorage) AddMessage(id int, message string) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.messages == nil {
		storage.messages = make(map[int]string)
	}
	storage.messages[id] = message
}

func (storage *MapMessageStorage) FindMessage(id int) (string, error) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	message, found := storage.messages[id]
	if !found {
		return "", fmt.Errorf("no message is associated with id %d", id)
	}
	return message, nil
}

func (storage *MapMessageStorage) MustFindMessage(id int) string {
	message, err := storage.FindMessage(id)
	if err != nil {
		panic(err)
	}
	return message
}
