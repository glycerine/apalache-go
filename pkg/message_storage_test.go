package apalache

import (
	"fmt"
	"sync"
	"testing"
)

func TestMessageStorageAddFindOverwriteAndMissing(t *testing.T) {
	storage := NewMessageStorage()
	if _, err := storage.FindMessage(7); err == nil {
		t.Fatal("found a missing message")
	}
	storage.AddMessage(7, "first")
	if got, err := storage.FindMessage(7); err != nil || got != "first" {
		t.Fatalf("message = %q, %v", got, err)
	}
	storage.AddMessage(7, "second")
	if got := storage.MustFindMessage(7); got != "second" {
		t.Fatalf("overwritten message = %q", got)
	}
	storage.AddMessage(-1, "negative")
	storage.AddMessage(0, "")
	if got, err := storage.FindMessage(-1); err != nil || got != "negative" {
		t.Fatalf("negative id message = %q, %v", got, err)
	}
	if got, err := storage.FindMessage(0); err != nil || got != "" {
		t.Fatalf("empty message = %q, %v", got, err)
	}
}

func TestMessageStorageMissingMustFindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustFindMessage did not mirror Scala missing-key failure")
		}
	}()
	NewMessageStorage().MustFindMessage(99)
}

func TestMessageStorageConcurrentIndependentIDs(t *testing.T) {
	storage := NewMessageStorage()
	var wait sync.WaitGroup
	for id := 0; id < 100; id++ {
		wait.Add(1)
		go func(id int) {
			defer wait.Done()
			storage.AddMessage(id, fmt.Sprintf("message-%d", id))
		}(id)
	}
	wait.Wait()
	for id := 0; id < 100; id++ {
		want := fmt.Sprintf("message-%d", id)
		if got, err := storage.FindMessage(id); err != nil || got != want {
			t.Fatalf("id %d = %q, %v; want %q", id, got, err, want)
		}
	}
}

func TestStratifiedRewriterImplementsMessageStorage(t *testing.T) {
	var storage MessageStorage = NewStratifiedRewriter()
	storage.AddMessage(42, "violation")
	if got, err := storage.FindMessage(42); err != nil || got != "violation" {
		t.Fatalf("rewriter message = %q, %v", got, err)
	}
}
