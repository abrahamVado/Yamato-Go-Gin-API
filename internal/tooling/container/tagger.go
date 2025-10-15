package container

import (
	"fmt"
	"regexp"
	"strings"
)

const registryHost = "ghcr.io"

var componentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// 1.- BuildImageName returns the GHCR repository for the provided component.
func BuildImageName(repository, component string) (string, error) {
	//1.- Validate that the GitHub repository follows the owner/name format.
	repo := strings.TrimSpace(repository)
	if repo == "" {
		return "", fmt.Errorf("repository is required")
	}
	if !strings.Contains(repo, "/") {
		return "", fmt.Errorf("repository must include owner and name")
	}
	repo = strings.Trim(repo, "/")
	repo = strings.ToLower(repo)

	//1.- Ensure the component name adheres to image naming constraints.
	name := strings.TrimSpace(component)
	name = strings.ToLower(name)
	if !componentPattern.MatchString(name) {
		return "", fmt.Errorf("component %q is invalid", component)
	}

	//1.- Assemble the fully-qualified image repository.
	return fmt.Sprintf("%s/%s/%s", registryHost, repo, name), nil
}

// 1.- BuildImageTags derives canonical tags for a component given the Git ref context.
func BuildImageTags(repository, component, gitRef, commitSHA, defaultBranch string) ([]string, error) {
	//1.- Reject empty commit hashes since tags must be unique per build.
	sha := strings.TrimSpace(commitSHA)
	if sha == "" {
		return nil, fmt.Errorf("commit SHA is required")
	}

	//2.- Produce the GHCR image name that all tags will share.
	image, err := BuildImageName(repository, component)
	if err != nil {
		return nil, err
	}

	//3.- Always include the commit hash tag to guarantee uniqueness.
	tags := make([]string, 0, 6)
	seen := map[string]struct{}{}
	addTag := func(tag string) {
		if tag == "" {
			return
		}
		if _, exists := seen[tag]; exists {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, fmt.Sprintf("%s:%s", image, tag))
	}
	addTag(truncateTag(sha))

	//4.- Detect branches and convert them to Docker-safe tag names.
	if strings.HasPrefix(gitRef, "refs/heads/") {
		branch := strings.TrimPrefix(gitRef, "refs/heads/")
		sanitized := sanitizeTag(branch)
		addTag(sanitized)
		if sanitized == sanitizeTag(defaultBranch) {
			addTag("latest")
		}
	}

	//5.- Convert semantic tags to multiple version aliases where applicable.
	if strings.HasPrefix(gitRef, "refs/tags/") {
		version := strings.TrimPrefix(gitRef, "refs/tags/")
		sanitized := sanitizeTag(version)
		addTag(sanitized)

		if strings.HasPrefix(version, "v") {
			trimmed := strings.TrimPrefix(version, "v")
			parts := strings.Split(trimmed, ".")
			switch {
			case len(parts) >= 3:
				addTag(sanitizeTag("v" + strings.Join(parts[:3], ".")))
				addTag(sanitizeTag("v" + strings.Join(parts[:2], ".")))
				addTag(sanitizeTag("v" + parts[0]))
			case len(parts) == 2:
				addTag(sanitizeTag("v" + strings.Join(parts[:2], ".")))
				addTag(sanitizeTag("v" + parts[0]))
			case len(parts) == 1:
				addTag(sanitizeTag("v" + parts[0]))
			}
		}
	}

	//6.- Publish a predictable tag for pull request builds.
	if strings.HasPrefix(gitRef, "refs/pull/") {
		remainder := strings.TrimPrefix(gitRef, "refs/pull/")
		tokens := strings.Split(remainder, "/")
		if len(tokens) > 0 {
			addTag(sanitizeTag("pr-" + tokens[0]))
		}
	}

	return tags, nil
}

// 1.- sanitizeTag converts arbitrary Git ref fragments into Docker tag safe strings.
func sanitizeTag(input string) string {
	//1.- Convert the candidate to lowercase to satisfy GHCR's canonical requirements.
	lowered := strings.ToLower(strings.TrimSpace(input))
	if lowered == "" {
		return ""
	}

	//2.- Replace unsupported characters with hyphens.
	builder := strings.Builder{}
	builder.Grow(len(lowered))
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('-')
	}

	//3.- Trim invalid leading or trailing separators.
	cleaned := strings.Trim(builder.String(), ".-")
	if cleaned == "" {
		return ""
	}

	//4.- Enforce Docker's 128-character limit for tags.
	const maxTagLength = 128
	if len(cleaned) > maxTagLength {
		cleaned = cleaned[:maxTagLength]
	}
	return cleaned
}

// 1.- truncateTag keeps the most informative portion of long SHAs within Docker tag limits.
func truncateTag(tag string) string {
	const maxTagLength = 128
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > maxTagLength {
		return trimmed[:maxTagLength]
	}
	return trimmed
}
