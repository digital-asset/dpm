// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assembler

import (
	"context"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/ocipuller"
	"daml.com/x/assistant/pkg/simpleplatform"
	"net/http/httptest"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func Fake(registry *httptest.Server) (*Assembler, error) {
	tmpDamlHome, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}

	config, err := assistantconfig.GetWithCustomDamlHome(tmpDamlHome)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}
	if registry != nil {
		puller := remotepuller.New(config,
			assistantremote.NewWithCustomClient(
				strings.TrimPrefix(registry.URL, "https://"),
				&auth.Client{Client: registry.Client()},
				false,
			),
		)
		return New(config, puller), nil
	}
	return New(config, &FakePuller{}), nil
}

type FakePuller struct{}

func (f *FakePuller) PullAssembly(ctx context.Context, edition sdkmanifest.Edition, tag, destPath string, platform *simpleplatform.NonGeneric) error {
	panic("not implemented")
}
func (f *FakePuller) PullComponent(ctx context.Context, componentName, tag, destPath string, platform simpleplatform.Platform) error {
	panic("not implemented")
}

var _ ocipuller.OciPuller = (*FakePuller)(nil)
