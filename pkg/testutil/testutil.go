// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocipusher"
	"daml.com/x/assistant/pkg/ocipusher/sdkmanifestpusher"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// TestdataPath gives absolute path within the common 'testdata'
func TestdataPath(t *testing.T, path ...string) string {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	p := []string{filepath.Dir(file), "testdata"}
	p = append(p, path...)
	return filepath.Join(p...)
}

func PushComponentUri(registry *httptest.Server, name, repo, tag, pathToComponent string, extraTags ...string) (args []string) {
	uri := fmt.Sprintf("%s/%s", GetRemote(registry).Registry, repo)

	args = []string{"publish", "component", "--name", name, "--version", tag, "--registry", "oci://" + uri, "-p", "generic=" + pathToComponent}

	if strings.HasPrefix(registry.URL, "http://") {
		args = append(args, "--insecure")
	}
	for _, t := range extraTags {
		args = append(args, "--extra-tags", t)
	}

	return args
}

func PushDarUri(registry *httptest.Server, name, repo, tag, pathToComponent string) (args []string) {
	uri := fmt.Sprintf("%s/%s", GetRemote(registry).Registry, repo)
	args = []string{"publish", "dar", "--name", name, "--version", tag,
		"-f", pathToComponent, "--registry", "oci://" + uri,
	}
	if strings.HasPrefix(registry.URL, "http://") {
		args = append(args, "--insecure")
	}

	return args
}

func PushComponent(t *testing.T, ctx context.Context, registry *httptest.Server, componentName, tag, pathToComponent string, extraTags ...string) {
	r := GetRemote(registry)
	v, err := semver.NewVersion(tag)
	require.NoError(t, err)
	requiredAnnotations := ociconsts.DescriptorAnnotations{
		Name:    componentName,
		Version: v,
	}
	opts := ocipusher.Opts{
		Artifact:            &ociconsts.FirstPartyComponentArtifact{ComponentName: componentName},
		RawTag:              tag,
		Dir:                 pathToComponent,
		RequiredAnnotations: requiredAnnotations,
		ExtraAnnotations:    map[string]string{},
		Platform:            &simpleplatform.Generic{},
	}
	pushOp, err := ocipusher.New(ctx, opts)
	require.NoError(t, err)
	desc, err := pushOp.Do(ctx, r)
	require.NoError(t, err)

	indexOpts := ociindex.Opts{
		Artifact:            &ociconsts.FirstPartyComponentArtifact{ComponentName: componentName},
		Tag:                 tag,
		Manifests:           []v1.Descriptor{*desc},
		ExtraAnnotations:    map[string]string{},
		RequiredAnnotations: requiredAnnotations,
	}
	_, err = ociindex.PushIndex(ctx, r, indexOpts)
	require.NoError(t, err)

	if len(extraTags) > 0 {
		err = ociindex.Tag(ctx, r, &ociconsts.FirstPartyComponentArtifact{ComponentName: componentName}, v, extraTags)
		require.NoError(t, err)
	}
}

func PushDar(t *testing.T, ctx context.Context, registry *httptest.Server, darName, tag, pathToComponent string) {
	r := GetRemote(registry)
	v, err := semver.NewVersion(tag)
	require.NoError(t, err)
	requiredAnnotations := ociconsts.DescriptorAnnotations{
		Name:    darName,
		Version: v,
	}
	opts := ocipusher.Opts{
		Artifact:            &ociconsts.DarArtifact{DarRepo: darName},
		RawTag:              tag,
		Dir:                 pathToComponent,
		RequiredAnnotations: requiredAnnotations,
		ExtraAnnotations:    map[string]string{},
		Platform:            &simpleplatform.Generic{},
	}
	pushOp, err := ocipusher.New(ctx, opts)
	require.NoError(t, err)
	desc, err := pushOp.Do(ctx, r)
	require.NoError(t, err)

	indexOpts := ociindex.Opts{
		Artifact:            &ociconsts.DarArtifact{DarRepo: darName},
		Tag:                 tag,
		Manifests:           []v1.Descriptor{*desc},
		ExtraAnnotations:    map[string]string{},
		RequiredAnnotations: requiredAnnotations,
	}
	_, err = ociindex.PushIndex(ctx, r, indexOpts)
	require.NoError(t, err)
}

// PushAssembly pushes assembly manifest to OCI registry for all platforms
func PushAssembly(t *testing.T, ctx context.Context, edition sdkmanifest.Edition, registry *httptest.Server, tag, pathToAssembly string) {
	platforms := []string{
		"windows/amd64",
		"linux/amd64",
		"darwin/amd64",
		"darwin/arm64",
	}

	r := GetRemote(registry)
	v, err := semver.NewVersion(tag)
	require.NoError(t, err)
	manifests := lo.SliceToMap(platforms, func(p string) (simpleplatform.NonGeneric, string) {
		platform, err := simpleplatform.ParsePlatform(p)
		require.NoError(t, err)
		require.False(t, platform.IsGeneric())
		nonGen := platform.(*simpleplatform.NonGeneric)
		return *nonGen, pathToAssembly
	})
	args := &sdkmanifestpusher.PushArgs{
		Edition:     edition,
		Version:     v,
		Annotations: map[string]string{},
		ExtraTags:   []string{"latest"},
	}
	_, err = sdkmanifestpusher.New(utils.StdPrinter{}, args).PushSdkManifest(ctx, r, manifests)
	require.NoError(t, err)
}

func GetRemote(registry *httptest.Server) *assistantremote.Remote {
	prefix := "http://"
	insecure := strings.HasPrefix(registry.URL, prefix)
	if !insecure {
		prefix = "https://"
	}
	return assistantremote.NewWithCustomClient(strings.TrimPrefix(registry.URL, prefix), &auth.Client{Client: registry.Client()}, insecure)
}

func StartRegistry(t *testing.T) (client *assistantremote.Remote, reg *httptest.Server) {
	reg = httptest.NewServer(registry.New())
	t.Cleanup(func() { reg.Close() })
	regUrl := strings.TrimPrefix(reg.URL, "http://")

	t.Setenv(assistantconfig.OciRegistryEnvVar, regUrl)
	t.Setenv(assistantconfig.RegistryAuthConfigPathEnvVar, TestdataPath(t, "empty-docker-config.json"))
	t.Setenv(assistantconfig.AllowInsecureRegistryEnvVar, "true")

	return GetRemote(reg), reg
}

type CommonSetupSuite struct {
	suite.Suite
}

func (suite *CommonSetupSuite) SetupTest() {
	// set DPM_HOME to a randomized temp dir before every test,
	// otherwise, the assistant will the same, default ~/.dpm across tests.

	tmpDpmHome, deleteFn, err := utils.MkdirTemp("", "")
	if err != nil {

	}
	suite.T().Setenv(assistantconfig.DpmHomeEnvVar, tmpDpmHome)
	suite.T().Cleanup(func() {
		deleteFn()
	})
}

func Context(t *testing.T) context.Context {
	ctx, stopFn := context.WithCancel(context.Background())
	t.Cleanup(stopFn)
	return ctx
}

var OS = func() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return "unix"
}()

func MustMarshal(t *testing.T, v any) []byte {
	t.Helper()

	b, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	return b
}
