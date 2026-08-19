// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/cacheindex"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"oras.land/oras-go/v2/registry"

	"daml.com/x/assistant/pkg/assistantversion"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

var (
	ErrNoSdkInstalled        = fmt.Errorf("dpm-sdk not installed")
	ErrTargetSdkNotInstalled = fmt.Errorf("target dpm-sdk version not installed")
)

type InstalledSdkVersion struct {
	Version      *semver.Version
	Edition      sdkmanifest.Edition
	ManifestPath string
}

func (i *InstalledSdkVersion) String() string {
	return fmt.Sprintf("%s %s", i.Version.String(), i.Edition.String())
}

type Config struct {
	DamlHomePath string `yaml:"-"`

	CachePath string `yaml:"-"`
	// oci-layout dir containing raw pulled blobs
	OciLayoutCache string `yaml:"-"`
	// dir containing the assembly manifests of installed dpm-sdks
	InstalledSdkManifestsPath string `yaml:"-"`

	InstallLocalFilePath string `yaml:"-"`

	AutoInstall bool `yaml:"auto-install,omitempty"`

	// Edition defaults to open-source
	Edition *LazyEdition `yaml:"edition,omitempty"`

	Registry         string `yaml:"registry,omitempty"`
	RegistryAuthPath string `yaml:"registry-auth-path,omitempty"`
	Insecure         bool   `yaml:"insecure,omitempty"`

	CacheIndex *cacheindex.CacheIndex `yaml:"-"`
}

func (c *Config) SdkManifestsRepo() (string, error) {
	e, err := c.Edition.Get()
	if err != nil {
		return "", err
	}
	return e.SdkManifestsRepo()
}

func (c *Config) EnsureDirs() error {
	err := utils.EnsureDirs(c.DamlHomePath, c.OciLayoutCache,
		filepath.Join(c.InstalledSdkManifestsPath, sdkmanifest.Enterprise.String()),
		filepath.Join(c.InstalledSdkManifestsPath, sdkmanifest.Private.String()),
		filepath.Join(c.InstalledSdkManifestsPath, sdkmanifest.OpenSource.String()))
	if err != nil {
		return err
	}

	return c.CacheIndex.Init()
}

func Get() (*Config, error) {
	dpmHomePath, err := getDamlHomePath()
	if err != nil {
		return nil, err
	}
	return GetWithCustomDamlHome(dpmHomePath)
}

func GetWithCustomDamlHome(dpmHomePath string) (*Config, error) {
	config := Config{}

	// dpm-config.yaml is optional
	configFilePath := filepath.Join(dpmHomePath, "dpm-config.yaml")
	fileInfo, err := os.Stat(configFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if fileInfo.IsDir() {
			return nil, fmt.Errorf("%q is directory and not a file", configFilePath)
		}

		bytes, err := os.ReadFile(configFilePath)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(bytes, &config); err != nil {
			return nil, err
		}
	}

	editionStr, ok := os.LookupEnv(EditionEnvVar)
	if ok {
		edition, err := sdkmanifest.ParseEdition(strings.ToLower(editionStr))
		if err != nil {
			return nil, err
		}
		config.Edition = NewLazyEdition(edition)
	} else if config.Edition == nil { // config file didn't provide it either, default to open-source
		config.Edition = NewLazyEdition(sdkmanifest.OpenSource)
	}

	autoInstall, ok, err := utils.BoolEnvVar(AutoInstallEnvVar)
	if err != nil {
		return nil, err
	}
	if ok {
		config.AutoInstall = autoInstall
	}

	registry, ok := os.LookupEnv(OciRegistryEnvVar)
	if ok {
		config.Registry = registry
	}
	if config.Registry == "" {
		// TODO: maybe don't default the registry here in the assistant,
		// and rather do so in the dpm-config.yaml instead?
		config.Registry = DefaultOciRegistry
	}

	registryAuthPath, ok := os.LookupEnv(RegistryAuthConfigPathEnvVar)
	if ok {
		config.RegistryAuthPath = registryAuthPath
	}

	insecure, ok, err := utils.BoolEnvVar(AllowInsecureRegistryEnvVar)
	if err != nil {
		return nil, err
	}
	if ok {
		config.Insecure = insecure
	}

	cacheDir := filepath.Join(dpmHomePath, "cache")
	config.DamlHomePath = dpmHomePath
	config.CachePath = cacheDir
	config.OciLayoutCache = filepath.Join(cacheDir, "oci-layout")
	config.InstalledSdkManifestsPath = filepath.Join(cacheDir, "sdk")
	config.InstallLocalFilePath = filepath.Join(config.InstalledSdkManifestsPath, ".lock")
	config.CacheIndex = &cacheindex.CacheIndex{
		AbsolutePath: filepath.Join(cacheDir, "index.json"),
	}
	return &config, nil
}

func getDamlHomePath() (string, error) {
	if v, ok := os.LookupEnv(DpmHomeEnvVar); ok {
		return v, nil
	}

	return getAppUserDataDirectory("dpm")
}

func getAppUserDataDirectory(appName string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		dir, ok := os.LookupEnv("APPDATA")
		if !ok {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}
		return filepath.Join(dir, appName), nil
	default:
		dir, ok := os.LookupEnv("HOME")
		if !ok {
			return "", fmt.Errorf("HOME environment variable is not set")
		}
		return filepath.Join(dir, "."+appName), nil
	}
}

// GetInstalledSdkVersion returns the specified installed sdk version, or ErrTargetSdkNotInstalled
func GetInstalledSdkVersion(config *Config, version *semver.Version) (*InstalledSdkVersion, error) {
	matchingVersion := func(vs []*InstalledSdkVersion) (*InstalledSdkVersion, bool) {
		return lo.Find(vs, func(v *InstalledSdkVersion) bool {
			return v.Version.Equal(version)
		})
	}
	v, ok, err := getInstalledSdkByPredicate(config, matchingVersion)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTargetSdkNotInstalled
	}
	return v, nil
}

// GetInstalledSdk returns the highest semver installed dpm-sdk, or ErrNoSdkInstalled
func GetInstalledSdk(config *Config) (*InstalledSdkVersion, error) {
	v, ok, err := getInstalledSdkByPredicate(config, lo.Last[*InstalledSdkVersion])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoSdkInstalled
	}
	return v, nil
}

// GetInstalledSdkFromEnvOrDefault returns the installed dpm-sdk specified in DPM_SDK_VERSION,
// or the highest semver one, or ErrNoSdkInstalled
func GetInstalledSdkFromEnvOrDefault(config *Config) (*InstalledSdkVersion, error) {
	override, ok := os.LookupEnv(DpmSdkVersionEnvVar)
	if !ok {
		return GetInstalledSdk(config)
	}

	matchesOverride := func(vs []*InstalledSdkVersion) (*InstalledSdkVersion, bool) {
		return lo.Find(vs, func(v *InstalledSdkVersion) bool {
			return v.Version.String() == override
		})
	}
	v, ok, err := getInstalledSdkByPredicate(config, matchesOverride)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, resolutionerrors.NewSdkNotInstalledError(fmt.Errorf("%w. You can install the needed sdk via 'dpm install %s'", ErrTargetSdkNotInstalled, override))
	}
	return v, nil

}

// GetSdkVersionOverrideWithFallback returns the DPM_SDK_VERSION if set
func GetSdkVersionOverrideWithFallback(fallback string) string {
	if v, ok := os.LookupEnv(DpmSdkVersionEnvVar); ok {
		return v
	}
	return fallback
}

// getInstalledSDKByPredicate returns the path to the assembly manifest
// of the installed dpm-sdk matching predicate, or ErrNoSdkInstalled.
func getInstalledSdkByPredicate(config *Config, chooseFn func(vs []*InstalledSdkVersion) (*InstalledSdkVersion, bool)) (*InstalledSdkVersion, bool, error) {
	sdkManifests, err := GetInstalledSDKsForEdition(config)
	if err != nil {
		return nil, false, err
	}
	v, ok := chooseFn(sdkManifests)
	if !ok {
		return nil, false, nil
	}
	return v, ok, nil
}

func GetInstalledSDKsForEdition(c *Config) ([]*InstalledSdkVersion, error) {
	edition, err := c.Edition.Get()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(c.InstalledSdkManifestsPath, edition.String())
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	type Tuple struct {
		Name    string
		Version *semver.Version
	}

	vs := lo.FilterMap(entries, func(e os.DirEntry, _ int) (*InstalledSdkVersion, bool) {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			return nil, false
		}
		v, err := semver.NewVersion(strings.TrimSuffix(e.Name(), ".yaml"))
		if err != nil {
			return nil, false
		}
		return &InstalledSdkVersion{
			Version:      v,
			Edition:      edition,
			ManifestPath: filepath.Join(p, e.Name()),
		}, true
	})
	slices.SortFunc(vs, func(a, b *InstalledSdkVersion) int {
		return a.Version.Compare(b.Version)
	})
	return vs, nil
}

// GetMultiPackageAbsolutePath returns true if the assistant was called in the scope of a multi-package.yaml,
// along with its absolute path
func GetMultiPackageAbsolutePath() (string, bool, error) {
	// DAML_MULTI_PACKAGE env var takes precedence
	multiPackagePath, ok := os.LookupEnv(DamlMultiPackageEnvVar)
	if ok {
		absolutePath, err := filepath.Abs(filepath.Join(multiPackagePath, DamlMultiPackageFilename))
		if err != nil {
			return "", false, err
		}
		return absolutePath, true, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	return findInAncestors(cwd, DamlMultiPackageFilename)
}

func GetDamlPackageAbsolutePath() (damlYamlAbsPath string, found bool, err error) {
	damlYamlAbsPath, found, err = getDamlPackageAbsolutePath()
	if err != nil {
		return "", false, fmt.Errorf("error looking for daml package: %w", err)
	}
	return
}

func getDamlPackageAbsolutePath() (string, bool, error) {
	// DAML_PACKAGE env var takes precedence
	damlYamlPath, ok := os.LookupEnv(DamlPackageEnvVar)
	if !ok {
		// then DAML_PROJECT (deprecated)
		damlYamlPath, ok = os.LookupEnv(DamlProjectEnvVar)
	}
	if ok {
		absolutePath, err := filepath.Abs(filepath.Join(damlYamlPath, DamlPackageFilename))
		if err != nil {
			return "", false, err
		}
		return absolutePath, true, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	return findInAncestors(cwd, DamlPackageFilename)
}

func findInAncestors(startDir, filename string) (absolutePath string, ok bool, err error) {
	p, ok, err := doFindInAncestors(startDir, filename)
	if err != nil {
		return
	}
	if !ok {
		return "", false, nil
	}
	absolutePath, err = filepath.Abs(p)
	return
}

func doFindInAncestors(startDir, filename string) (string, bool, error) {
	f := filepath.Join(startDir, filename)

	info, err := os.Stat(f)
	if err == nil && !info.IsDir() {
		return f, true, nil
	}

	parent := filepath.Dir(startDir)
	if parent == startDir {
		return "", false, nil
	}

	return doFindInAncestors(parent, filename)
}

func GetAssistantUserAgent() string {
	return fmt.Sprintf("%s/%s", AssistantUserAgentPrefix, assistantversion.GetAssistantVersion())
}

func DpmLockfileEnabled() bool {
	return os.Getenv(DpmLockfileEnabledEnvVar) == "true"
}

func ShaPinningEnabled() bool {
	return true
}

func (c *Config) CachePathForDar(ref *registry.Reference) string {
	if ShaPinningEnabled() {
		sha := strings.ReplaceAll(ref.Reference, "sha256:", "")
		return filepath.Join(c.CachePath, "dars", "sha256", sha)
	}
	return filepath.Join(c.CachePath, "dars", utils.UrlToFilePath(fmt.Sprintf("%s/%s", ref.Registry, ref.Repository)), ref.Reference)
}

func safeGitCacheSegment(name, value string) (string, error) {
	if value == "" || value == "." || value == ".." {
		return "", fmt.Errorf("invalid git cache %s segment %q", name, value)
	}
	if filepath.IsAbs(value) || strings.ContainsAny(value, `/\`) {
		return "", fmt.Errorf("invalid git cache %s segment %q", name, value)
	}
	return value, nil
}

func gitRepoSegments(cloneURL *url.URL) (host, org, repo string, err error) {
	if cloneURL == nil {
		return "", "", "", fmt.Errorf("missing git clone URL")
	}

	host = cloneURL.Host
	if cloneURL.Scheme == "file" {
		host = "local"
	}
	host, err = safeGitCacheSegment("host", host)
	if err != nil {
		return "", "", "", err
	}

	repoPath := strings.TrimPrefix(cloneURL.Path, "/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	parts := strings.Split(repoPath, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", "", fmt.Errorf("missing git repository path")
	}
	for _, part := range parts {
		if _, err := safeGitCacheSegment("repo path", part); err != nil {
			return "", "", "", err
		}
	}

	if len(parts) == 1 {
		return host, parts[0], parts[0], nil
	}
	org = strings.Join(parts[:len(parts)-1], "/")
	repo = parts[len(parts)-1]
	return host, org, repo, nil
}

// CachePathForGit returns ~/.dpm/cache/git/<host>/<org>/<repo>/<ref-hash>/path/to/foo.dar
func (c *Config) CachePathForGit(host, org, repo, refHash, darPath string) (string, error) {
	for name, value := range map[string]string{
		"host": host,
		"repo": repo,
		"ref":  refHash,
	} {
		if _, err := safeGitCacheSegment(name, value); err != nil {
			return "", err
		}
	}
	for part := range strings.SplitSeq(org, "/") {
		if _, err := safeGitCacheSegment("org", part); err != nil {
			return "", err
		}
	}

	cleanDarPath := filepath.ToSlash(filepath.Clean(darPath))
	if cleanDarPath == "." || cleanDarPath == ".." || strings.HasPrefix(cleanDarPath, "../") || filepath.IsAbs(darPath) {
		return "", fmt.Errorf("invalid git cache dar path %q", darPath)
	}

	return filepath.Join(c.CachePath, "git", host, org, repo, refHash, filepath.FromSlash(cleanDarPath)), nil
}

// CachePathForGitDependency resolves cache segments from a clone URL and ref hash.
func (c *Config) CachePathForGitDependency(cloneURL *url.URL, darPath, refHash string) (string, error) {
	host, org, repo, err := gitRepoSegments(cloneURL)
	if err != nil {
		return "", err
	}
	return c.CachePathForGit(host, org, repo, refHash, darPath)
}

// GitWorkPathForRepo returns the clone work directory for a git repository.
func (c *Config) GitWorkPathForRepo(cloneURL *url.URL) (string, error) {
	host, org, repo, err := gitRepoSegments(cloneURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(c.CachePath, "git", host, org, repo, ".work"), nil
}

// CachePathForGitRelease returns the cached .dar path for a git release asset.
func (c *Config) CachePathForGitRelease(cloneURL *url.URL, releaseTag, asset string) (string, error) {
	host, org, repo, err := gitRepoSegments(cloneURL)
	if err != nil {
		return "", err
	}
	refHash := sha256Hex(releaseTag + "\x00" + asset)
	return c.CachePathForGit(host, org, repo, refHash, asset)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
