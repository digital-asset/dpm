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

var regex = regexp.MustCompile(`^(@[a-zA-Z0-9_-]+)/`)

func (p *DamlPackage) parseLocations(ds []string, artifactLocations ArtifactLocations) (map[string]*ParsedDarDependency, error) {
	parsedLocations := map[string]*ParsedDarDependency{}

	var errs []error

	for _, d := range ds {
		if strings.HasPrefix(d, "oci://") {
			u, err := url.Parse(d)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse dependency url %q: %w", d, err))
				continue
			}
			parsedLocations[d] = &ParsedDarDependency{FullUrl: u}
		} else if strings.HasPrefix(d, "http://") || strings.HasPrefix(d, "https://") {
			// TODO
			errs = append(errs, fmt.Errorf("couldn't parse dependency %q: http dependencies not yet supported", d))
			continue
		} else if strings.HasSuffix(d, ".dar") {
			absPath := utils.ResolvePath(filepath.Dir(p.AbsolutePath), d)
			u, err := url.Parse("file://" + filepath.ToSlash(absPath))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			parsedLocations[d] = &ParsedDarDependency{
				Location: nil,
				FullUrl:  u,
			}
		} else if strings.HasPrefix(d, "@") {
			parsed := regex.FindStringSubmatch(d)
			if len(parsed) < 2 {
				errs = append(errs, fmt.Errorf("error parsing dependency %q: Dependencies beginning with @ must be of the form '@<artifact-location>/<suffix>'", d))
				continue
			}
			location, ok := artifactLocations[parsed[1]]
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
			parsedLocations[d] = &ParsedDarDependency{
				Location: location,
				FullUrl:  u,
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
				Location: nil,
				FullUrl:  u,
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return parsedLocations, nil
}
