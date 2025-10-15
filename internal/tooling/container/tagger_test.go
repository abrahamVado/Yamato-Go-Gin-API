package container

import (
	"sort"
	"testing"
)

// 1.- TestBuildImageNameNormalizesRepository ensures owner/name pairs are lowercased and scoped to GHCR.
func TestBuildImageNameNormalizesRepository(t *testing.T) {
	//1.- Invoke the helper with a mixed-case repository and component.
	image, err := BuildImageName("Example/Yamato", "API")
	if err != nil {
		t.Fatalf("BuildImageName returned error: %v", err)
	}

	//2.- Verify the output repository is fully qualified and normalized.
	expected := "ghcr.io/example/yamato/api"
	if image != expected {
		t.Fatalf("expected %s, got %s", expected, image)
	}
}

// 1.- TestBuildImageTagsForDefaultBranch includes latest and branch tags for main deployments.
func TestBuildImageTagsForDefaultBranch(t *testing.T) {
	//1.- Generate tags for a commit on the default branch.
	tags, err := BuildImageTags("example/yamato", "api", "refs/heads/main", "1234567890abcdef", "main")
	if err != nil {
		t.Fatalf("BuildImageTags returned error: %v", err)
	}

	//2.- Sort tags for deterministic comparison.
	sort.Strings(tags)
	expected := []string{
		"ghcr.io/example/yamato/api:1234567890abcdef",
		"ghcr.io/example/yamato/api:latest",
		"ghcr.io/example/yamato/api:main",
	}
	sort.Strings(expected)

	//3.- Compare tag sets for equality.
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d", len(expected), len(tags))
	}
	for i := range tags {
		if tags[i] != expected[i] {
			t.Fatalf("unexpected tag at %d: got %s want %s", i, tags[i], expected[i])
		}
	}
}

// 1.- TestBuildImageTagsForSemverEmitsAliases verifies semantic versions receive patch, minor, and major aliases.
func TestBuildImageTagsForSemverEmitsAliases(t *testing.T) {
	//1.- Produce tags for a release ref.
	tags, err := BuildImageTags("example/yamato", "worker", "refs/tags/v1.2.3", "abcdef012345", "main")
	if err != nil {
		t.Fatalf("BuildImageTags returned error: %v", err)
	}

	//2.- Sort to simplify assertions.
	sort.Strings(tags)
	expected := []string{
		"ghcr.io/example/yamato/worker:abcdef012345",
		"ghcr.io/example/yamato/worker:v1",
		"ghcr.io/example/yamato/worker:v1.2",
		"ghcr.io/example/yamato/worker:v1.2.3",
	}
	sort.Strings(expected)

	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d", len(expected), len(tags))
	}
	for i := range tags {
		if tags[i] != expected[i] {
			t.Fatalf("unexpected tag at %d: got %s want %s", i, tags[i], expected[i])
		}
	}
}

// 1.- TestBuildImageTagsForPullRequestUsesPRPrefix emits a predictable tag for pull request refs.
func TestBuildImageTagsForPullRequestUsesPRPrefix(t *testing.T) {
	//1.- Evaluate a pull request ref to ensure the pr-N format is applied.
	tags, err := BuildImageTags("example/yamato", "api", "refs/pull/42/merge", "0f1e2d3c4b5a", "main")
	if err != nil {
		t.Fatalf("BuildImageTags returned error: %v", err)
	}

	//2.- Sort results for deterministic comparison.
	sort.Strings(tags)
	expected := []string{
		"ghcr.io/example/yamato/api:0f1e2d3c4b5a",
		"ghcr.io/example/yamato/api:pr-42",
	}
	sort.Strings(expected)

	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d", len(expected), len(tags))
	}
	for i := range tags {
		if tags[i] != expected[i] {
			t.Fatalf("unexpected tag at %d: got %s want %s", i, tags[i], expected[i])
		}
	}
}
