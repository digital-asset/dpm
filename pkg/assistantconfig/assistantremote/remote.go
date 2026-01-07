// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantremote

import (
	"fmt"
	"log/slog"
	"net/http"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote/readonlydynamicstore"
	"daml.com/x/assistant/pkg/ocicache"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type Remote struct {
	Registry string
	client   *auth.Client

	// Use http instead of https.
	// This is merely a hint to consumers of Remote, and not something that is enforced by Client
	Insecure bool
}

func (r *Remote) Repo(repoName string) (repo *remote.Repository, err error) {
	repo, err = remote.NewRepository(fmt.Sprintf("%s/%s", r.Registry, repoName))
	if err != nil {
		return nil, err
	}

	repo.Client = r
	repo.PlainHTTP = r.Insecure
	return
}

func (r *Remote) CachedRepo(repoName, ociCache string) (oras.ReadOnlyTarget, error) {
	var repo oras.ReadOnlyTarget
	repo, err := r.Repo(repoName)
	if err != nil {
		return nil, err
	}
	return ocicache.CachedTarget(repo, ociCache)
}

func NewWithCustomClient(registry string, client *auth.Client, insecure bool) *Remote {
	return &Remote{
		Registry: registry,
		client:   client,
		Insecure: insecure,
	}
}

func New(registry string, authConfigPath string, insecure bool) (*Remote, error) {
	// This client has some default caching (e.g. for auth tokens) and retry settings
	client := auth.DefaultClient
	client.SetUserAgent(assistantconfig.GetAssistantUserAgent())

	if authConfigPath != "" {
		slog.Info("using custom auth for registry", "path", authConfigPath)
		ds, err := credentials.NewStore(authConfigPath, credentials.StoreOptions{})
		if err != nil {
			return nil, err
		}
		client.Credential = credentials.Credential(readonlydynamicstore.New(ds))
	} else {
		slog.Debug("no custom registry auth provided. Will default to docker's if present on system")
		ds, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
		if err != nil {
			slog.Debug("failed to determine docker config to default to. Requests to registry will be unauthenticated", "err", err.Error())
		} else {
			client.Credential = credentials.Credential(readonlydynamicstore.New(ds))
		}
	}

	return &Remote{
		Registry: registry,
		client:   client,
		Insecure: insecure,
	}, nil
}

var _ remote.Client = (*Remote)(nil)

func (c *Remote) Do(req *http.Request) (*http.Response, error) {
	slog.Debug("OCI request", "method", req.Method, "url", req.URL.String())
	return c.client.Do(req)
}

func NewFromConfig(config *assistantconfig.Config) (*Remote, error) {
	return New(config.Registry, config.RegistryAuthPath, config.Insecure)
}
