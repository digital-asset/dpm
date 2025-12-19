// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package login

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"github.com/jdx/go-netrc"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type loginCmd struct {
	username, password, netrcHost string
	passwordStdin, useNativeStore bool
}

func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := &loginCmd{}
	cmd := &cobra.Command{
		Use: string(builtincommand.Login),
		Long: "Authenticate to the registry. This will modify the auth config file. " +
			"(The registry and auth file are the ones specified in " +
			"dpm-config.yaml or the corresponding env vars, or the defaults if none are set)",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := c.get()
			if err != nil {
				return err
			}

			if err := c.login(cmd.Context(), config, creds); err != nil {
				return err
			}

			cmd.Println("Successfully logged in.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&c.username, "username", "u", "", "username")
	cmd.Flags().StringVarP(&c.password, "password", "p", "", "password")
	cmd.Flags().BoolVar(&c.passwordStdin, "password-stdin", false, "Take the password from stdin")
	cmd.Flags().BoolVar(&c.useNativeStore, "use-native-cred-store", false, "store credentials in system's credential store instead of plaintext in the auth config file")
	cmd.Flags().StringVarP(&c.netrcHost, "netrc", "n", "", "log in using username and password of a netrc host (machine)")

	return cmd
}

func (c *loginCmd) storeOpts() credentials.StoreOptions {
	return credentials.StoreOptions{
		AllowPlaintextPut:        true,
		DetectDefaultNativeStore: c.useNativeStore,
	}
}

func (c *loginCmd) login(ctx context.Context, config *assistantconfig.Config, creds *auth.Credential) (err error) {
	var authConfigPath string
	var ds *credentials.DynamicStore

	if config.RegistryAuthPath != "" {
		authConfigPath = config.RegistryAuthPath
		ds, err = credentials.NewStore(config.RegistryAuthPath, c.storeOpts())
	} else {
		authConfigPath = "docker's config.json"
		ds, err = credentials.NewStoreFromDocker(c.storeOpts())
	}
	slog.Debug("login parameters", "auth-config-path", authConfigPath, "registry", config.Registry)
	if err != nil {
		return err
	}

	regUrl := config.Registry
	if !strings.HasPrefix(regUrl, "http://") && !strings.HasPrefix(regUrl, "https://") {
		regUrl = "http://" + regUrl
	}
	u, err := url.Parse(regUrl)
	if err != nil {
		return err
	}

	reg, err := remote.NewRegistry(u.Host)
	if err != nil {
		return err
	}
	return credentials.Login(ctx, ds, reg, *creds)
}

func (c *loginCmd) get() (*auth.Credential, error) {
	if c.netrcHost != "" {
		if c.username != "" || c.passwordStdin {
			return nil, fmt.Errorf("netrc can't be used with other options")
		}

		return getNetrc(c.netrcHost)
	}

	if c.username == "" {
		return nil, fmt.Errorf("username is required")
	}

	if c.passwordStdin && c.password != "" {
		return nil, fmt.Errorf("--password and --password-stdin cannot be used together")
	}

	if !c.passwordStdin && c.password == "" {
		return nil, fmt.Errorf("password must be provided via --password or --passowrdStdin")
	}

	if c.password != "" {
		return &auth.Credential{
			Username: c.username,
			Password: c.password,
		}, nil
	}

	// stdin
	p, err := io.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		return nil, fmt.Errorf("failed to read password from stdin: %w", err)
	}
	return &auth.Credential{
		Username: c.username,
		Password: string(p),
	}, nil
}

func getNetrc(host string) (*auth.Credential, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	n, err := netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
	if err != nil {
		return nil, err
	}

	machine := n.Machine(host)
	return &auth.Credential{
		Username: machine.Get("login"),
		Password: machine.Get("password"),
	}, nil
}
