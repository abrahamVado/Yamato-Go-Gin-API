package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubSettingsProvider struct {
	settings FileUploadSettings
	err      error
}

func (s stubSettingsProvider) FileUploadSettings(context.Context) (FileUploadSettings, error) {
	if s.err != nil {
		return FileUploadSettings{}, s.err
	}
	return s.settings, nil
}

// 1.- TestSaveRejectsDisallowedMIME ensures the allowlist is honoured strictly.
func TestSaveRejectsDisallowedMIME(t *testing.T) {
	t.Parallel()

	//2.- Prepare a file service limited to PNG uploads.
	provider := stubSettingsProvider{settings: FileUploadSettings{
		AllowedMIMETypes: []string{"image/png"},
		MaxUploadSize:    1024,
	}}
	svc, err := NewFileService("local", t.TempDir(), provider)
	if err != nil {
		t.Fatalf("unexpected error creating file service: %v", err)
	}

	//3.- Attempt to upload a JPEG and expect the MIME check to fail.
	_, err = svc.Save(context.Background(), "example.jpg", bytes.NewReader([]byte("data")), int64(len("data")), "image/jpeg")
	if !errors.Is(err, ErrMIMENotAllowed) {
		t.Fatalf("expected ErrMIMENotAllowed, got %v", err)
	}
}

// 1.- TestSaveEnforcesSizeLimit verifies the size ceiling is respected when streaming data.
func TestSaveEnforcesSizeLimit(t *testing.T) {
	t.Parallel()

	//2.- Configure a tiny size limit to trigger the guard.
	provider := stubSettingsProvider{settings: FileUploadSettings{
		AllowedMIMETypes: []string{"text/plain"},
		MaxUploadSize:    4,
	}}
	svc, err := NewFileService("local", t.TempDir(), provider)
	if err != nil {
		t.Fatalf("unexpected error creating file service: %v", err)
	}

	//3.- Build a payload larger than the declared limit and ensure it is rejected.
	payload := bytes.Repeat([]byte("a"), 8)
	_, err = svc.Save(context.Background(), "oversized.txt", bytes.NewReader(payload), int64(len(payload)), "text/plain")
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

// 1.- TestSaveGeneratesDeterministicPath checks the path layout and sanitisation logic.
func TestSaveGeneratesDeterministicPath(t *testing.T) {
	t.Parallel()

	//2.- Build a deterministic clock so the resulting path is predictable.
	fixedTime := time.Date(2024, 3, 15, 10, 30, 0, 123456000, time.UTC)
	provider := stubSettingsProvider{settings: FileUploadSettings{
		AllowedMIMETypes: []string{"text/plain"},
		MaxUploadSize:    32,
	}}
	root := t.TempDir()
	svc, err := NewFileService("local", root, provider)
	if err != nil {
		t.Fatalf("unexpected error creating file service: %v", err)
	}
	svc.WithClock(func() time.Time { return fixedTime })

	//3.- Save a file with path traversal attempts in the name.
	data := []byte("hello world")
	rel, err := svc.Save(context.Background(), "../../weird name.txt", bytes.NewReader(data), int64(len(data)), "text/plain")
	if err != nil {
		t.Fatalf("unexpected error saving file: %v", err)
	}

	//4.- Verify the returned path matches the expected date hierarchy and sanitised name.
	expectedDir := filepath.Join("2024", "03", "15")
	if filepath.Dir(rel) != expectedDir {
		t.Fatalf("expected directory %s, got %s", expectedDir, filepath.Dir(rel))
	}
	if !strings.HasSuffix(rel, "_weird-name.txt") {
		t.Fatalf("expected sanitised suffix, got %s", rel)
	}

	//5.- Confirm the file exists on disk and contains the right payload.
	fullPath := filepath.Join(root, rel)
	contents, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read stored file: %v", err)
	}
	if !bytes.Equal(contents, data) {
		t.Fatalf("expected %q, got %q", data, contents)
	}
}

// 1.- TestGenerateDownloadURLProvidesPlaceholder validates the temporary signed URL implementation.
func TestGenerateDownloadURLProvidesPlaceholder(t *testing.T) {
	t.Parallel()

	//2.- Prepare a service and override the clock for predictable expiries.
	provider := stubSettingsProvider{settings: FileUploadSettings{}}
	svc, err := NewFileService("local", t.TempDir(), provider)
	if err != nil {
		t.Fatalf("unexpected error creating file service: %v", err)
	}
	svc.WithClock(func() time.Time { return time.Unix(0, 0) })

	//3.- Request a signed URL with a custom TTL and check the response metadata.
	url, err := svc.GenerateDownloadURL(context.Background(), "foo/bar.txt", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating placeholder URL: %v", err)
	}
	if url.URL != "/files/local/foo/bar.txt?signature=todo" {
		t.Fatalf("unexpected URL returned: %s", url.URL)
	}
	if !url.Expires.Equal(time.Unix(0, 0).Add(time.Minute)) {
		t.Fatalf("unexpected expiry: %s", url.Expires)
	}
}

// 1.- TestSanitiseFileNameNormalisesUnsupportedCharacters ensures names remain predictable.
func TestSanitiseFileNameNormalisesUnsupportedCharacters(t *testing.T) {
	t.Parallel()

	//2.- Provide a name containing path traversal and punctuation.
	name := sanitiseFileName("../Tricky File(1).txt")

	//3.- Confirm the resulting name is lowercased and stripped of illegal characters.
	if name != "tricky-file-1-.txt" {
		t.Fatalf("unexpected sanitised name: %s", name)
	}
}

// 1.- TestCopyWithLimitStopsAtThreshold ensures the helper enforces the byte limit accurately.
func TestCopyWithLimitStopsAtThreshold(t *testing.T) {
	t.Parallel()

	//2.- Stream a payload larger than the allowed number of bytes.
	src := bytes.NewReader(bytes.Repeat([]byte("x"), 10))
	var buf bytes.Buffer
	n, err := copyWithLimit(&buf, src, 4)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}

	//3.- The helper must stop at the limit and store the expected number of bytes.
	if n != 4 {
		t.Fatalf("expected to copy 4 bytes, got %d", n)
	}
	if buf.String() != "xxxx" {
		t.Fatalf("unexpected buffer contents: %s", buf.String())
	}

	//4.- The reader should still contain enough data to confirm truncation occurred.
	remaining, err := io.ReadAll(src)
	if err != nil {
		t.Fatalf("unexpected error reading remainder: %v", err)
	}
	if len(remaining) != 5 {
		t.Fatalf("expected 5 bytes remaining, got %d", len(remaining))
	}
}
