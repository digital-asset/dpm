package damlpackage

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type ArtifactLocations map[string]*ArtifactLocation

type ArtifactLocation struct {
	Url     string `yaml:"url"`
	Default bool   `yaml:"default"`
	Auth    string `yaml:"auth"`
}

type ResolvedDependency struct {
	// the fully-qualified URL for the artifact e.g. oci://example.com/foo/bar/baz:1.2.3
	FullUrl *url.URL

	// can be nil when the corresponding dependency is already fully qualified and doesn't rely on an artifact-location
	Location *ArtifactLocation
}

var regex = regexp.MustCompile(`^(@[a-zA-Z0-9_-]+)/[^/]+$`)

func (ls ArtifactLocations) GetDefaultLocation() (name string, location *ArtifactLocation, err error) {
	for s, l := range ls {
		if l.Default {
			if name != "" {
				return "", nil, fmt.Errorf("only one artifact location can be set as default")
			}
			name = s
			location = l
		}
	}
	return
}

func (p *DamlPackage) computeResolvedDependencies(defaultLocation *ArtifactLocation) (map[string]*ResolvedDependency, error) {
	resolved := map[string]*ResolvedDependency{}

	var errs []error

	for _, d := range p.Dependencies {
		if strings.HasPrefix(d, "oci://") {
			u, err := url.Parse(d)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse dependency url %q: %w", d, err))
				continue
			}
			resolved[d] = &ResolvedDependency{FullUrl: u}
		} else if strings.HasPrefix(d, "@") {
			parsed := regex.FindStringSubmatch(d)
			if len(parsed) < 2 {
				errs = append(errs, fmt.Errorf("error parsing dependency %q: Dependencies beginning with @ must be of the form '@<artifact-location>/<suffix>'", d))
				continue
			}
			location, ok := p.ArtifactLocations[parsed[1]]
			if !ok {
				errs = append(errs, fmt.Errorf("dependency %q has no corresponding artifact location", d))
				continue
			}

			if location.Url == "" {
				errs = append(errs, fmt.Errorf("invalid artifact location %q. Must have a non-empty url", location.Url))
				continue
			}

			rawUrl := strings.Replace(d, parsed[1], location.Url, 1)
			u, err := url.Parse(rawUrl)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse full url %q for dependency %q: ", rawUrl, d))
				continue
			}
			resolved[d] = &ResolvedDependency{
				Location: location,
				FullUrl:  u,
			}
		} else if strings.Contains(d, ":") {
			if defaultLocation == nil {
				errs = append(errs, fmt.Errorf("failed to resolve dependency's artifact location for %q: no default artifact location is specified", d))
				continue
			}

			rawUrl := fmt.Sprintf("%s/%s", defaultLocation.Url, d)
			u, err := url.Parse(rawUrl)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse full url %q for dependency %q: ", rawUrl, d))
				continue
			}
			resolved[d] = &ResolvedDependency{
				Location: defaultLocation,
				FullUrl:  u,
			}
		} else {
			// non-remote dependency
			resolved[d] = nil
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return resolved, nil
}
