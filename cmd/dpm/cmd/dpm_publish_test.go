// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/testutil"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *MainSuite) TestPublishComponentWithAppendIndexFlag() {
	t := suite.T()
	client, reg := testutil.StartRegistry(t)

	repo := "foo/meep"
	uri := fmt.Sprintf("oci://%s/%s:1.2.3", testutil.GetRemote(reg).Registry, repo)

	publish := func(extraArgs ...string) error {
		args := append([]string{"publish", "component", uri, "--insecure"}, extraArgs...)
		return createStdTestRootCmd(t, args...).Execute()
	}
	unix := "linux/amd64=" + testutil.TestdataPath(t, "meepy-component", "unix")
	windows := "windows/amd64=" + testutil.TestdataPath(t, "meepy-component", "windows")

	require.NoError(t, publish("-p", unix))
	require.NoError(t, publish("-p", windows, "--append-index"))

	index, _, err := ociindex.FetchIndex(t.Context(), client, repo, "1.2.3")
	require.NoError(t, err)

	platforms := lo.Map(index.Manifests, func(d v1.Descriptor, _ int) string {
		return fmt.Sprintf("%s/%s", d.Platform.OS, d.Platform.Architecture)
	})
	assert.ElementsMatch(t, []string{"linux/amd64", "windows/amd64"}, platforms)
}
