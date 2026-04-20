package componentlist

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/registry"
)

type ComponentList []string

func (compList ComponentList) GenerateAsMap() (map[string]*sdkmanifest.Component, error) {
	var compMap map[string]*sdkmanifest.Component
	var errs []error

	for _, c := range compList {
		if strings.HasPrefix(c, "oci://") { // oci://whatever.dev/foo/bar/comp:1.2.3
			u, err := registry.ParseReference(strings.TrimPrefix(c, "oci://"))
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse component url %q: %w", c, err))
				continue
			}
			name, _ := lo.Last(strings.Split(u.Registry, "/"))
			uri := u.String()

			compMap[name] = &sdkmanifest.Component{Name: name, Uri: &uri}
		} else if strings.HasPrefix(c, "http://") || strings.HasPrefix(c, "https://") {
			// TODO
			errs = append(errs, fmt.Errorf("couldn't parse component %q: http not yet supported", c))
			continue
		} else if strings.HasPrefix(c, ".") {
			// TODO
			errs = append(errs, fmt.Errorf("couldn't parse component %q: file paths not yet supported", c))
			continue
		} else if strings.HasPrefix(c, "@") {
			errs = append(errs, fmt.Errorf("couldn't parse component %q: aliases not yet supported", c))
			continue
		} else if strings.Contains(c, ":") && !strings.Contains(c, "/") {
			// e.g. "damlc:1.2.3"

			parts := strings.Split(c, ":")
			name, version := parts[0], parts[1]

			semVer, err := semver.StrictNewVersion(version)
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse component %q: %w", c, err))
				continue
			}

			compMap[name] = &sdkmanifest.Component{Name: name, Version: sdkmanifest.AssemblySemVer(semVer)}
		} else {
			errs = append(errs, fmt.Errorf("couldn't parse component %q: must be of the form <name>:<version> or oci://<uri>:<version>", c))
		}
	}
	return compMap, nil
}
