package semver

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
)

type StrictSemVer semver.Version

func New(s string) (*StrictSemVer, error) {
	v, err := semver.StrictNewVersion(s)
	if err != nil {
		return nil, err
	}
	sv := StrictSemVer(*v)
	return &sv, nil
}

func (v *StrictSemVer) Value() semver.Version {
	return (semver.Version)(*v)
}

func (v *StrictSemVer) UnmarshalYAML(data []byte) error {
	var versionStr string
	if err := yaml.Unmarshal(data, &versionStr); err != nil {
		return fmt.Errorf("failed to unmarshal 'version': %w", err)
	}
	parsedVersion, err := semver.StrictNewVersion(versionStr)
	if err != nil {
		return fmt.Errorf("invalid semantic version: %w", err)
	}
	*v = StrictSemVer(*parsedVersion)
	return nil
}

func (v *StrictSemVer) MarshalYAML() ([]byte, error) {
	return []byte(v.Value().String()), nil
}

var _ yaml.BytesUnmarshaler = (*StrictSemVer)(nil)
var _ yaml.BytesMarshaler = (*StrictSemVer)(nil)
