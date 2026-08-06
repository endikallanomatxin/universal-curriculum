package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ObjectInfo struct {
	StorageKey string
	SizeBytes  int64
	ETag       *string
}

type StoredObject struct {
	io.ReadCloser
	Info ObjectInfo
}

type ObjectStore interface {
	Put(ctx context.Context, storageKey string, content io.Reader) (ObjectInfo, error)
	Get(ctx context.Context, storageKey string) (*StoredObject, error)
	Delete(ctx context.Context, storageKey string) error
}

type LocalObjectStore struct {
	root string
}

func NewLocalObjectStore(root string) *LocalObjectStore {
	return &LocalObjectStore{root: root}
}

func (store *LocalObjectStore) Put(ctx context.Context, storageKey string, content io.Reader) (ObjectInfo, error) {
	absolutePath, err := buildLocalObjectPath(store.root, storageKey)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return ObjectInfo{}, fmt.Errorf("create object directory: %w", err)
	}

	destination, err := os.OpenFile(absolutePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("create object: %w", err)
	}
	defer destination.Close()

	sizeBytes, err := copyWithContext(ctx, destination, content)
	if err != nil {
		_ = os.Remove(absolutePath)
		return ObjectInfo{}, fmt.Errorf("write object: %w", err)
	}
	return ObjectInfo{StorageKey: storageKey, SizeBytes: sizeBytes}, nil
}

func (store *LocalObjectStore) Get(ctx context.Context, storageKey string) (*StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absolutePath, err := buildLocalObjectPath(store.root, storageKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("open object: %w", err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return &StoredObject{
		ReadCloser: file,
		Info:       ObjectInfo{StorageKey: storageKey, SizeBytes: stat.Size()},
	}, nil
}

func (store *LocalObjectStore) Delete(ctx context.Context, storageKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	absolutePath, err := buildLocalObjectPath(store.root, storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(absolutePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func buildLocalObjectPath(root, storageKey string) (string, error) {
	cleanRoot := filepath.Clean(root)
	absolutePath := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(storageKey)))
	if absolutePath == cleanRoot {
		return "", fmt.Errorf("invalid object storage key")
	}
	if !strings.HasPrefix(absolutePath, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid object storage path")
	}
	return absolutePath, nil
}
