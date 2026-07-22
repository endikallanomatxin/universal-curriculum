package services

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLocalObjectStoreLifecycle(t *testing.T) {
	store := NewLocalObjectStore(t.TempDir())
	ctx := context.Background()

	info, err := store.Put(ctx, "units/example/lesson.txt", strings.NewReader("curriculum"))
	if err != nil {
		t.Fatal(err)
	}
	if info.StorageKey != "units/example/lesson.txt" || info.SizeBytes != 10 {
		t.Fatalf("unexpected object info: %+v", info)
	}

	object, err := store.Get(ctx, info.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(object)
	if closeErr := object.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "curriculum" || object.Info.SizeBytes != 10 {
		t.Fatalf("unexpected stored object: info=%+v content=%q", object.Info, content)
	}

	if err := store.Delete(ctx, info.StorageKey); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, info.StorageKey); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Get() after Delete() error = %v, want os.ErrNotExist", err)
	}
	if err := store.Delete(ctx, info.StorageKey); err != nil {
		t.Fatalf("repeated Delete() error = %v", err)
	}
}

func TestLocalObjectStoreRejectsInvalidKeys(t *testing.T) {
	store := NewLocalObjectStore(t.TempDir())
	for _, key := range []string{"", ".", "../outside", "nested/../../outside"} {
		if _, err := store.Put(context.Background(), key, strings.NewReader("content")); err == nil {
			t.Errorf("Put(%q) unexpectedly succeeded", key)
		}
	}
}

func TestLocalObjectStoreDoesNotOverwriteObjects(t *testing.T) {
	store := NewLocalObjectStore(t.TempDir())
	if _, err := store.Put(context.Background(), "same-key", strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "same-key", strings.NewReader("second")); err == nil {
		t.Fatal("second Put() unexpectedly succeeded")
	}
}

func TestLocalObjectStoreHonoursCancellation(t *testing.T) {
	store := NewLocalObjectStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Put(ctx, "cancelled", strings.NewReader("content")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() error = %v, want context.Canceled", err)
	}
	if _, err := store.Get(ctx, "cancelled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context.Canceled", err)
	}
}
