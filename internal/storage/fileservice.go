package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrProviderUnsupported is returned when a non-local backend is requested.
var ErrProviderUnsupported = errors.New("only the local storage provider is supported")

// ErrMIMENotAllowed is returned when attempting to store a file with a disallowed type.
var ErrMIMENotAllowed = errors.New("mime type not allowed")

// ErrFileTooLarge is returned when the payload exceeds the configured limit.
var ErrFileTooLarge = errors.New("file exceeds maximum allowed size")

// FileUploadSettings describes the validation constraints retrieved from the settings store.
type FileUploadSettings struct {
	AllowedMIMETypes []string
	MaxUploadSize    int64
}

// FileSettingsProvider exposes the configuration lookup required by the file service.
type FileSettingsProvider interface {
	FileUploadSettings(ctx context.Context) (FileUploadSettings, error)
}

// SignedURL represents a placeholder response for future integrations with secure URL generation.
type SignedURL struct {
	URL     string
	Expires time.Time
}

// FileService persists files to the local filesystem while honouring dynamic settings.
type FileService struct {
	root      string
	provider  FileSettingsProvider
	now       func() time.Time
	filePerms os.FileMode
}

// NewFileService validates the configuration and prepares the local storage backend.
func NewFileService(providerName string, localPath string, settingsProvider FileSettingsProvider) (*FileService, error) {
	//1.- Ensure the service runs in local mode because remote providers are not implemented yet.
	if strings.TrimSpace(strings.ToLower(providerName)) != "local" {
		return nil, ErrProviderUnsupported
	}

	//2.- Require a destination path so uploads are stored in a deterministic location.
	if strings.TrimSpace(localPath) == "" {
		return nil, errors.New("local storage path is required")
	}

	//3.- Validate that the settings provider exists because limits are driven dynamically.
	if settingsProvider == nil {
		return nil, errors.New("file settings provider is required")
	}

	//4.- Resolve the absolute path and create the directory tree if it does not already exist.
	cleaned := filepath.Clean(localPath)
	if err := os.MkdirAll(cleaned, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	//5.- Return a fully configured file service bound to the local filesystem.
	return &FileService{
		root:      cleaned,
		provider:  settingsProvider,
		now:       time.Now,
		filePerms: 0o640,
	}, nil
}

// WithClock overrides the internal clock, simplifying deterministic testing scenarios.
func (s *FileService) WithClock(now func() time.Time) {
	//1.- Swap the time source so callers can produce stable paths and expiry timestamps.
	if now != nil {
		s.now = now
	}
}

// Save stores the file after enforcing MIME allowlists and size limits from dynamic settings.
func (s *FileService) Save(ctx context.Context, originalName string, content io.Reader, size int64, mimeType string) (string, error) {
	//1.- Guard against incorrect instantiation or missing dependencies.
	if s == nil || s.provider == nil {
		return "", errors.New("file service is not initialized")
	}
	if content == nil {
		return "", errors.New("content reader is required")
	}

	//2.- Fetch the latest validation settings from the backing store.
	settings, err := s.provider.FileUploadSettings(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load file settings: %w", err)
	}

	//3.- Normalise the MIME type and ensure it is present in the allowlist when configured.
	normalisedMIME := strings.ToLower(strings.TrimSpace(mimeType))
	if len(settings.AllowedMIMETypes) > 0 {
		allowed := false
		for _, candidate := range settings.AllowedMIMETypes {
			if normalisedMIME == strings.ToLower(strings.TrimSpace(candidate)) && normalisedMIME != "" {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", ErrMIMENotAllowed
		}
	}

	//4.- Enforce the configured size ceiling before touching the filesystem.
	if settings.MaxUploadSize > 0 && size > settings.MaxUploadSize {
		return "", ErrFileTooLarge
	}

	//5.- Generate the relative path that deterministically organises files by date.
	ts := s.now().UTC()
	safeName := sanitiseFileName(originalName)
	relPath := filepath.Join(
		fmt.Sprintf("%04d", ts.Year()),
		fmt.Sprintf("%02d", ts.Month()),
		fmt.Sprintf("%02d", ts.Day()),
		fmt.Sprintf("%d_%s", ts.UnixNano(), safeName),
	)
	fullPath := filepath.Join(s.root, relPath)

	//6.- Make sure the parent directory exists before writing the payload to disk.
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", fmt.Errorf("failed to prepare storage directory: %w", err)
	}

	//7.- Create the destination file using restrictive permissions.
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, s.filePerms)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	//8.- Stream the payload and ensure the effective size never exceeds the configured threshold.
	if settings.MaxUploadSize > 0 {
		if _, err = copyWithLimit(file, content, settings.MaxUploadSize); errors.Is(err, ErrFileTooLarge) {
			_ = os.Remove(fullPath)
			return "", ErrFileTooLarge
		} else if err != nil {
			_ = os.Remove(fullPath)
			return "", fmt.Errorf("failed to write file: %w", err)
		}
	} else {
		if _, err = io.Copy(file, content); err != nil {
			_ = os.Remove(fullPath)
			return "", fmt.Errorf("failed to write file: %w", err)
		}
	}

	//9.- Return the relative path so callers can persist references without leaking absolute paths.
	return relPath, nil
}

// GenerateDownloadURL returns a placeholder signed URL for future HTTP integrations.
func (s *FileService) GenerateDownloadURL(ctx context.Context, relativePath string, ttl time.Duration) (SignedURL, error) {
	//1.- Require a path so the future signer can derive the canonical resource representation.
	if strings.TrimSpace(relativePath) == "" {
		return SignedURL{}, errors.New("relative path is required")
	}

	//2.- Default the TTL to five minutes when callers omit the value to ensure short-lived URLs.
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	//3.- Produce a deterministic placeholder that HTTP handlers can expose while the signer is unimplemented.
	expiry := s.now().Add(ttl)
	url := fmt.Sprintf("/files/local/%s?signature=todo", relativePath)

	//4.- Return the synthetic URL and expiry metadata.
	return SignedURL{URL: url, Expires: expiry}, nil
}

// copyWithLimit streams data from the reader honouring the provided byte ceiling.
func copyWithLimit(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	//1.- When no limit is configured fall back to the standard copy implementation.
	if limit <= 0 {
		return io.Copy(dst, src)
	}

	//2.- Stream at most `limit` bytes into the destination.
	limited := &io.LimitedReader{R: src, N: limit}
	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, err
	}

	//3.- Probe the reader to detect additional data without persisting it to disk.
	if limited.N == 0 {
		probe := make([]byte, 1)
		n, readErr := src.Read(probe)
		if n > 0 {
			return written, ErrFileTooLarge
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return written, readErr
		}
	}

	//4.- Return the number of bytes written which can be used for bookkeeping.
	return written, nil
}

// sanitiseFileName removes path traversal attempts and replaces unsupported characters.
func sanitiseFileName(name string) string {
	//1.- Strip directory components and fall back to a generic name when empty.
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "file"
	}

	//2.- Replace separator characters to prevent escaping the storage root.
	base = strings.ReplaceAll(base, string(filepath.Separator), "-")

	//3.- Allow a conservative character set and convert everything else to dashes.
	builder := strings.Builder{}
	builder.Grow(len(base))
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('-')
		}
	}

	//4.- Collapse sequences of dashes to keep filenames compact and readable.
	cleaned := builder.String()
	cleaned = strings.Trim(cleaned, "-")
	for strings.Contains(cleaned, "--") {
		cleaned = strings.ReplaceAll(cleaned, "--", "-")
	}
	if cleaned == "" {
		cleaned = "file"
	}

	//5.- Lowercase the name to normalise storage paths.
	return strings.ToLower(cleaned)
}
