package damlpackage

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/utils"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type ArtifactLocations map[string]*ArtifactLocation

type ArtifactLocation struct {
	Alias    string `yaml:"-"`
	Url      string `yaml:"url"`
	Auth     string `yaml:"auth"`
	Insecure bool   `yaml:"insecure"`

	// For unit tests
	Client *auth.Client
}

type ParsedDarDependency struct {
	// the fully-qualified URL for the artifact e.g. oci://example.com/foo/bar/baz:1.2.3
	FullUrl *url.URL

	// can be nil when the corresponding dependency is already fully qualified and doesn't rely on an artifact-location
	Location *ArtifactLocation

	GitRef     string
	DarPath    string
	CloneURL   *url.URL
	GitRelease bool

	MainPackageId *string

	// the index of this dependency in the list in daml.yaml.
	// useful when trying to update the dep as part of `dpm update`.
	Index int
}

func (d *ParsedDarDependency) Scheme() string {
	if d.FullUrl == nil {
		return ""
	}
	return d.FullUrl.Scheme
}

// StringWithAlias reconstructs the original '@<alias>/...' form for OCI dars.
func (d *ParsedDarDependency) StringWithAlias() string {
	if d.FullUrl == nil {
		return ""
	}
	u := d.FullUrl.String()

	if d.Location == nil || !strings.HasPrefix(u, d.Location.Url) {
		return u
	}

	return d.Location.Alias + strings.TrimPrefix(u, d.Location.Url)
}

func (d *ParsedDarDependency) GetOciRemote() (*assistantremote.Remote, *registry.Reference, error) {
	ref, err := registry.ParseReference(strings.TrimPrefix(d.FullUrl.String(), "oci://"))
	if err != nil {
		return nil, nil, err
	}

	insecure := d.Location != nil && d.Location.Insecure

	var assistantRemote *assistantremote.Remote
	if d.Location != nil && d.Location.Client != nil {
		assistantRemote = assistantremote.NewWithCustomClient(ref.Registry, d.Location.Client, insecure)
	} else {
		auth := ""
		if d.Location != nil {
			auth = d.Location.Auth
		}
		assistantRemote, err = assistantremote.New(ref.Registry, auth, insecure)
		if err != nil {
			return nil, nil, err
		}
	}

	return assistantRemote, &ref, nil
}

// artifactLocationAliasRegex captures '@alias' at the start of a dependency entry.
var artifactLocationAliasRegex = regexp.MustCompile(`^(@[a-zA-Z0-9_-]+)`)

func (p *DamlPackage) parseLocations(rawDeps []*RawDependency, artifactLocations ArtifactLocations) (map[string]*ParsedDarDependency, error) {
	parsedLocations := map[string]*ParsedDarDependency{}

	var errs []error

	for i, rawDep := range rawDeps {
		d, err := rawDep.Value()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if strings.HasPrefix(d, "oci://") {
			u, err := url.Parse(d)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse dependency url %q: %w", d, err))
				continue
			}
			parsedLocations[d] = &ParsedDarDependency{
				FullUrl:       u,
				MainPackageId: rawDep.GetMainPackageId(),
				Index:         i,
			}
		} else if IsGitDependencyLine(d) {
			gitDep, err := ParseGitDependency(d)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			gitDep.MainPackageId = rawDep.GetMainPackageId()
			gitDep.Index = i
			parsedLocations[d] = gitDep
		} else if strings.HasPrefix(d, "http://") || strings.HasPrefix(d, "https://") {
			// TODO
			errs = append(errs, fmt.Errorf("couldn't parse dependency %q: http dependencies not yet supported", d))
			continue
		} else if strings.HasPrefix(d, "@") {
			alias, location, err := lookupArtifactLocation(d, artifactLocations)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if isGitLocationURL(location.Url) {
				if strings.ContainsAny(location.Url, "#?") {
					errs = append(errs, fmt.Errorf("git artifact location %q must be a bare repository URL (no #ref or query); put the #ref and ?path=/?release= on the dependency line", alias))
					continue
				}
				gitDep, err := ParseGitDependency(strings.Replace(d, alias, location.Url, 1))
				if err != nil {
					errs = append(errs, fmt.Errorf("dependency %q: %w", d, err))
					continue
				}
				gitDep.Location = location
				gitDep.MainPackageId = rawDep.GetMainPackageId()
				gitDep.Index = i
				parsedLocations[d] = gitDep
				continue
			}

			if !strings.HasPrefix(strings.TrimPrefix(d, alias), "/") {
				errs = append(errs, fmt.Errorf("error parsing dependency %q: OCI dependencies beginning with @ must be of the form '@<artifact-location>/<suffix>'", d))
				continue
			}

			rawUrl := strings.Replace(d, alias, location.Url, 1)
			u, err := url.Parse(rawUrl)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse full url %q for dependency %q: ", rawUrl, d))
				continue
			}
			parsedLocations[d] = &ParsedDarDependency{
				Location:      location,
				FullUrl:       u,
				MainPackageId: rawDep.GetMainPackageId(),
				Index:         i,
			}
		} else if IsDarPath(d) {
			absPath := utils.ResolvePath(filepath.Dir(p.AbsolutePath), d)
			u, err := url.Parse("file://" + filepath.ToSlash(absPath))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			parsedLocations[d] = &ParsedDarDependency{
				Location:      nil,
				FullUrl:       u,
				MainPackageId: rawDep.GetMainPackageId(),
				Index:         i,
			}
		} else if strings.Contains(d, ":") {
			errs = append(errs, fmt.Errorf("error parsing dependency %q: OCI dependencies must start with oci://", d))
			continue
		} else {
			// builtin libs (like "daml-script")

			s := "builtin://" + d
			u, err := url.Parse(s)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse full url %q for dependency %q: ", s, d))
				continue
			}
			parsedLocations[d] = &ParsedDarDependency{
				Location:      nil,
				FullUrl:       u,
				MainPackageId: rawDep.GetMainPackageId(),
				Index:         i,
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return parsedLocations, nil
}

func lookupArtifactLocation(dep string, artifactLocations ArtifactLocations) (alias string, location *ArtifactLocation, err error) {
	parsed := artifactLocationAliasRegex.FindStringSubmatch(dep)
	if len(parsed) < 2 {
		return "", nil, fmt.Errorf("error parsing dependency %q: dependencies beginning with @ must use an artifact-locations alias", dep)
	}
	alias = parsed[1]
	location, ok := artifactLocations[alias]
	if !ok {
		return "", nil, fmt.Errorf("dependency %q has no corresponding artifact location", dep)
	}
	if location.Url == "" {
		return "", nil, fmt.Errorf("invalid artifact location %q: must have a non-empty url", alias)
	}
	return alias, location, nil
}

func isGitLocationURL(locationURL string) bool {
	return strings.HasPrefix(locationURL, "git:")
}

// ArtifactLocationAlias returns the '@alias' prefix of a dependency entry, if any.
func ArtifactLocationAlias(dep string) (string, bool) {
	parsed := artifactLocationAliasRegex.FindStringSubmatch(dep)
	if len(parsed) < 2 {
		return "", false
	}
	return parsed[1], true
}
