// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"context"
	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assembler/assemblyplan"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/component"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/resolver"
	"daml.com/x/assistant/pkg/sdkinstall"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/utils"
	"errors"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
)

type DamlAssistant struct {
	Stderr, Stdout, Stdin *os.File
	ExitFn                func(exitCode int)
	// must contain at least one argument, namely the dpm binary name, similar to os.Args
	OsArgs []string
}

type resolutionType struct {
	value string
}

var (
	ShallowResolution = resolutionType{"shallow"}
	DeepResolution    = resolutionType{"deep"}
)

func (da *DamlAssistant) SetOutputStreams(cmd *cobra.Command) {
	cmd.SetOut(da.Stdout)
	cmd.SetErr(da.Stderr)
	cmd.SetIn(da.Stdin)

	lo.ForEach(cmd.Commands(), func(sub *cobra.Command, _ int) {
		da.SetOutputStreams(sub)
	})
}

func (da *DamlAssistant) ComputeSdkCommandsFromAssemblyManifest(ctx context.Context, config *assistantconfig.Config, manifst *sdkmanifest.SdkManifest) ([]*cobra.Command, error) {
	return da.computeSdkCommands(ctx, config, func(a *assembler.Assembler) (map[string][]*assembler.ValidatedCommand, string, error) {
		// TODO "dpm component run" command doesn't yet support daml.yaml or multi-package.yaml
		result, err := a.Assemble(ctx, manifst)
		if err != nil {
			return nil, "", err
		}
		return result.ValidatedCommands, "", nil
	})
}

func (da *DamlAssistant) ComputeSdkCommandsFromAssemblyPlan(ctx context.Context, config *assistantconfig.Config, resolutionType resolutionType) ([]*cobra.Command, error) {
	return da.computeSdkCommands(ctx, config, func(a *assembler.Assembler) (map[string][]*assembler.ValidatedCommand, string, error) {
		var deepResolutionFilePath string
		if resolutionType == DeepResolution {
			deepResolution, err := resolver.New(config, a).RunDeepResolution(ctx)
			if err != nil {
				return nil, "", err
			}

			deepResolutionFilePath, err = writeDeepResolutionFile(deepResolution)
			if err != nil {
				return nil, "", err
			}
			slog.Debug("deep resolution file written", "path", deepResolutionFilePath)
		}

		assemblyPlan, err := assemblyplan.New(ctx, config, a)
		if err != nil {
			return nil, "", err
		}
		result, err := assemblyPlan.Assemble(ctx)
		if err != nil {
			return nil, "", err
		}

		return result.ValidatedCommands, deepResolutionFilePath, nil
	})
}

// writeDeepResolutionFile writes the resolution to a file and returns the path to it
func writeDeepResolutionFile(deepResolution *resolution.Resolution) (string, error) {
	bytes, err := yaml.Marshal(deepResolution)
	if err != nil {
		return "", err
	}

	f, err := createResolutionFile()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(bytes); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func createResolutionFile() (*os.File, error) {
	customPath, ok := os.LookupEnv(assistantconfig.ResolutionFilePathEnvVar)
	if !ok {
		return os.CreateTemp("", "*.yaml")
	}

	if err := utils.EnsureDirs(filepath.Dir(customPath)); err != nil {
		slog.Error("failed to create dir for resolution file", "file", customPath, "error", err)
		return nil, err
	}
	return os.OpenFile(customPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
}

func (da *DamlAssistant) computeSdkCommands(ctx context.Context, config *assistantconfig.Config, getValidatedCommands func(*assembler.Assembler) (map[string][]*assembler.ValidatedCommand, string, error)) ([]*cobra.Command, error) {
	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		return nil, err
	}

	a := assembler.New(config, puller)
	a.DependencyPathWarnOnly = true
	a.ExportsPathsWarnOnly = true

	validatedCmds, deepResolutionFilePath, err := getValidatedCommands(a)
	if err != nil {
		return nil, err
	}
	return da.toCobraCommands(ctx, config, lo.Flatten(lo.Values(validatedCmds)), deepResolutionFilePath)
}

func (da *DamlAssistant) toCobraCommands(execContext context.Context, config *assistantconfig.Config, cmds []*assembler.ValidatedCommand, deepResolutionFilePath string) ([]*cobra.Command, error) {
	dpmBin, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dpmPath := sdkinstall.GetLinkTarget(config, dpmBin)

	damlYamlAbsPath, isDamlPkg, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}

	return lo.Map(cmds, func(c *assembler.ValidatedCommand, _ int) *cobra.Command {
		cmd := &cobra.Command{
			Use:                c.GetName(),
			DisableFlagParsing: true,
			SilenceUsage:       true,
			Aliases:            c.GetAliases(),
			RunE: func(cmd *cobra.Command, args []string) error {
				var binaryPath string
				var fullArgs []string

				switch v := c.Command.(type) {
				case *component.JarCommand:
					binaryPath = "java"
					fullArgs = append(fullArgs, v.JvmArgs...)
					fullArgs = append(fullArgs, "-jar")
					fullArgs = append(fullArgs, c.AbsolutePath)
					fullArgs = append(fullArgs, v.JarArgs...)
					fullArgs = append(fullArgs, args...)
				case *component.NativeCommand:
					binaryPath = c.AbsolutePath
					fullArgs = append(fullArgs, v.ExecArgs...)
					fullArgs = append(fullArgs, args...)
				}

				extraEnv := map[string]string{
					assistantconfig.DpmPathInjectedEnvVar: dpmPath,
				}
				if c.ResolvedDependencies != nil {
					maps.Copy(extraEnv, c.ResolvedDependencies)
				}

				extraEnv[assistantconfig.ResolutionFilePathEnvVar] = deepResolutionFilePath
				if c.SdkVersion != nil {
					// when a dpm.local.yaml is being used, there won't be an SDK in effect
					extraEnv[assistantconfig.DpmSdkVersionEnvVar] = c.SdkVersion.String()
				}

				// inject DAML_PACKAGE env var into command for their convenience
				if isDamlPkg {
					extraEnv[assistantconfig.DamlPackageEnvVar] = filepath.Dir(damlYamlAbsPath)
				}

				exitCode, err := da.execSdkCommand(execContext, binaryPath, fullArgs, extraEnv)
				if err != nil {
					return err
				}
				da.ExitFn(exitCode)
				return nil
			},
		}
		if c.GetDesc() != nil {
			cmd.Short = *c.GetDesc()
		} else {
			cmd.Hidden = true
		}
		return cmd
	}), nil
}

func (da *DamlAssistant) execSdkCommand(ctx context.Context, path string, args []string, extraEnv map[string]string) (int, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = da.Stdin
	cmd.Stdout = da.Stdout
	cmd.Stderr = da.Stderr

	env := lo.MapToSlice(extraEnv, func(key string, value string) string {
		return fmt.Sprintf("%s=%s", key, value)
	})
	env = append(env, os.Environ()...)
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode(), nil
		} else {
			return 0, fmt.Errorf("failed to spawn command subprocess. %w", err)
		}
	}
	return 0, nil
}
