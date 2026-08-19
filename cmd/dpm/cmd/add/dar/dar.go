package dar

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	project "daml.com/x/assistant/cmd/dpm/cmd/install/package"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/githubrelease"
	"daml.com/x/assistant/pkg/gitpuller"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/yamledit"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var insecure bool
	var dependencies, dataDependencies bool

	cmd := &cobra.Command{
		Use:   "dar <oci-uri|git-uri> <--dependencies | --data-dependencies>",
		Short: "add or update a dar in the project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			uri := args[0]

			normalizedURI, isGit, err := damlpackage.NormalizeDarDependencyURI(uri)
			if err != nil {
				return err
			}
			if isGit {
				uri = normalizedURI
			}

			depsFieldName, err := dependenciesFieldFromArgs(dependencies, dataDependencies)
			if err != nil {
				return err
			}

			damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("must be in daml.yaml directory or sub-directory")
			}

			// figure out if we need to update rather than add
			existingDep, err := findExistingDependency(uri, depsFieldName)
			if err != nil {
				return err
			}

			yamlTarget := yamledit.YamlTarget{
				YamlFilePath: damlPackagePath,
				FieldName:    depsFieldName,
				Index:        -1,
			}

			// update
			if existingDep != nil {
				yamlTarget.Index = existingDep.Index
				if damlpackage.IsGitDependencyLine(uri) {
					fmt.Printf("dependency %q already exists in daml.yaml, will be updated...\n", uri)
				} else {
					ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
					if err != nil {
						return err
					}
					fmt.Printf("dependency 'oci://%s/%s' already exists in daml.yaml, will be updated...\n", ref.Registry, ref.Reference)
				}
				return AddOrUpdateDar(ctx, config, uri, insecure, yamlTarget)
			}

			// add
			return AddOrUpdateDar(ctx, config, uri, insecure, yamlTarget)
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().BoolVar(&dependencies, "dependencies", false, "add the dar to the dependencies field")
	cmd.Flags().BoolVar(&dataDependencies, "data-dependencies", false, "add the dar to the data-dependencies field")

	return cmd
}

func findExistingDependency(uri, depsFieldName string) (*damlpackage.ParsedDarDependency, error) {
	normalizedURI, isGit, err := damlpackage.NormalizeDarDependencyURI(uri)
	if err != nil {
		return nil, err
	}
	if isGit {
		uri = normalizedURI
	}

	damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("must be in daml.yaml directory or sub-directory")
	}

	damlPackage, err := damlpackage.Read(damlPackagePath)
	if err != nil {
		return nil, err
	}

	deps := damlPackage.ParsedDarDependencies.Dependencies
	if depsFieldName == "data-dependencies" {
		deps = damlPackage.ParsedDarDependencies.DataDependencies
	}

	var parsedGitKey string
	var uriRef registry.Reference
	if damlpackage.IsGitDependencyLine(uri) {
		parsedGit, err := damlpackage.ParseGitDependency(uri)
		if err != nil {
			return nil, err
		}
		parsedGitKey, err = damlpackage.GitLockKeyForDep(parsedGit)
		if err != nil {
			return nil, err
		}
	} else {
		parsed, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
		if err != nil {
			return nil, err
		}
		uriRef = parsed
	}

	for _, dep := range deps {
		if damlpackage.IsGitDependencyLine(uri) {
			if dep.Scheme() == "git" {
				depKey, err := damlpackage.GitLockKeyForDep(dep)
				if err != nil {
					return nil, err
				}
				if depKey == parsedGitKey {
					return dep, nil
				}
			}
		} else if dep.FullUrl != nil && dep.FullUrl.Scheme == "oci" {
			depUrl := dep.FullUrl.String()

			depRef, err := registry.ParseReference(strings.TrimPrefix(depUrl, "oci://"))
			if err != nil {
				return nil, fmt.Errorf("invalid uri %q in daml.yaml or multi-package.yaml: %w", depUrl, err)
			}

			if uriRef.Registry == depRef.Registry && uriRef.Repository == depRef.Repository {
				return dep, nil
			}
		}
	}

	return nil, nil
}

// AddOrUpdateDar will add when yamlTarget.Index is -1, otherwise it will update at that index.
func AddOrUpdateDar(ctx context.Context, config *assistantconfig.Config, uri string, insecure bool, yamlTarget yamledit.YamlTarget) error {
	normalizedURI, isGit, err := damlpackage.NormalizeDarDependencyURI(uri)
	if err != nil {
		return err
	}
	if isGit {
		uri = normalizedURI
	}

	if damlpackage.IsGitDependencyLine(uri) {
		return addOrUpdateGitDar(ctx, config, uri, yamlTarget)
	}
	return addOrUpdateOciDar(ctx, config, uri, insecure, yamlTarget)
}

func addOrUpdateOciDar(ctx context.Context, config *assistantconfig.Config, uri string, insecure bool, yamlTarget yamledit.YamlTarget) error {
	ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	if err != nil {
		return err
	}
	client, err := assistantremote.New(ref.Registry, "", insecure)
	if err != nil {
		return err
	}

	// Resolve to sha256
	resolvedDigest, manifest, err := ocilister.FetchManifest(ctx, client, ref)
	if err != nil {
		return err
	}
	target := yamlTarget.Copy()
	target.LineComment = manifest.Annotations[v1.AnnotationVersion]
	resolvedUri := uri + "@" + resolvedDigest.String()

	parsedUrl, err := url.Parse(resolvedUri)
	if err != nil {
		return err
	}
	parsedDarDep := &damlpackage.ParsedDarDependency{
		FullUrl: parsedUrl,
		Location: &damlpackage.ArtifactLocation{
			Insecure: insecure,
		},
	}
	if _, _, err := project.InstallDar(ctx, config, parsedDarDep); err != nil {
		return err
	}

	if err := yamledit.EditYaml(target, resolvedUri); err != nil {
		return err
	}

	fmt.Printf("Successfully installed and added dar %q to %q\n", resolvedUri, target.YamlFilePath)
	return nil
}

func addOrUpdateGitDar(ctx context.Context, config *assistantconfig.Config, uri string, yamlTarget yamledit.YamlTarget) error {
	dep, err := damlpackage.ParseGitDependency(uri)
	if err != nil {
		return err
	}

	if dep.GitRelease && strings.TrimSpace(dep.DarPath) == "" {
		return addOrUpdateGitReleaseDar(ctx, config, dep, yamlTarget)
	}

	return addOrUpdateSingleGitDar(ctx, config, uri, yamlTarget)
}

func addOrUpdateGitReleaseDar(ctx context.Context, config *assistantconfig.Config, dep *damlpackage.ParsedDarDependency, yamlTarget yamledit.YamlTarget) (retErr error) {
	damlPackagePath := yamlTarget.YamlFilePath

	if err := githubrelease.ValidateReleaseHost(dep.CloneURL); err != nil {
		return err
	}

	undoBaseLineWrite := false
	if yamlTarget.Index == -1 {
		original, err := os.ReadFile(damlPackagePath)
		if err != nil {
			return err
		}
		baseLine := damlpackage.FormatGitReleaseBaseLine(dep)
		quotedLine := fmt.Sprintf("\"%s\"", baseLine)
		if err := yamledit.EditYaml(yamlTarget, quotedLine); err != nil {
			return err
		}
		undoBaseLineWrite = true
		defer func() {
			if retErr != nil && undoBaseLineWrite {
				_ = os.WriteFile(damlPackagePath, original, 0644)
			}
		}()
	}

	damlPackage, err := damlpackage.Read(damlPackagePath)
	if err != nil {
		return err
	}

	var rawDeps []*damlpackage.RawDependency
	if yamlTarget.FieldName == "data-dependencies" {
		rawDeps = damlPackage.DataDependencies
	} else {
		rawDeps = damlPackage.Dependencies
	}

	expanded, err := damlpackage.ExpandGitReleaseDependenciesInYaml(ctx, damlPackagePath, yamlTarget.FieldName, rawDeps)
	if err != nil {
		return err
	}
	undoBaseLineWrite = false
	if expanded {
		damlPackage, err = damlpackage.Read(damlPackagePath)
		if err != nil {
			return err
		}
	}

	var parsedDeps map[string]*damlpackage.ParsedDarDependency
	if yamlTarget.FieldName == "data-dependencies" {
		parsedDeps = damlPackage.ParsedDarDependencies.DataDependencies
	} else {
		parsedDeps = damlPackage.ParsedDarDependencies.Dependencies
	}

	baseKey, err := damlpackage.GitLockKeyForDep(dep)
	if err != nil {
		return err
	}

	var releaseDeps []*damlpackage.ParsedDarDependency
	for _, d := range parsedDeps {
		if !d.GitRelease || strings.TrimSpace(d.DarPath) == "" {
			continue
		}
		expBase := *d
		expBase.DarPath = ""
		expBaseKey, err := damlpackage.GitLockKeyForDep(&expBase)
		if err != nil || expBaseKey != baseKey {
			continue
		}
		releaseDeps = append(releaseDeps, d)
	}

	if n, err := gitpuller.FetchMissingReleaseAssets(ctx, config, releaseDeps); err != nil {
		return err
	} else if n > 0 {
		fmt.Printf("Fetched %d git release assets\n", n)
	}

	for _, d := range releaseDeps {
		line := damlpackage.FormatGitYamlLine(d)
		fmt.Printf("Successfully installed and added dar %q to %q\n", line, yamlTarget.YamlFilePath)
	}

	return nil
}

func addOrUpdateSingleGitDar(ctx context.Context, config *assistantconfig.Config, uri string, yamlTarget yamledit.YamlTarget) error {
	dep, err := damlpackage.ParseGitDependency(uri)
	if err != nil {
		return err
	}

	result, err := gitpuller.PullGitDar(ctx, config, dep)
	if err != nil {
		return err
	}

	pinnedLine := damlpackage.FormatGitYamlLine(dep)
	if result.Pinned != nil {
		pinnedLine = damlpackage.FormatGitYamlLine(result.Pinned)
	}

	target := yamlTarget.Copy()
	var raw *damlpackage.RawDependency
	if existing, err := findExistingDependency(uri, target.FieldName); err != nil {
		return err
	} else if existing != nil {
		target.Index = existing.Index
		pkg, err := damlpackage.Read(target.YamlFilePath)
		if err != nil {
			return err
		}
		rawDeps := pkg.Dependencies
		if target.FieldName == "data-dependencies" {
			rawDeps = pkg.DataDependencies
		}
		if target.Index >= 0 && target.Index < len(rawDeps) {
			raw = rawDeps[target.Index]
		}
	}

	item := fmt.Sprintf("%q", pinnedLine)
	if raw != nil {
		item, err = damlpackage.MarshalDependencyWithValue(raw, pinnedLine)
		if err != nil {
			return err
		}
	}
	if err := yamledit.EditYaml(target, item); err != nil {
		return err
	}

	fmt.Printf("Successfully installed and added dar %q to %q\n", pinnedLine, target.YamlFilePath)
	return nil
}

func dependenciesFieldFromArgs(dependencies, dataDependencies bool) (string, error) {
	if dataDependencies && dependencies {
		return "", fmt.Errorf("--dependencies and --data-dependencies cannot both be provided")
	}
	if dependencies {
		return "dependencies", nil
	}
	if dataDependencies {
		return "data-dependencies", nil
	}
	return "", fmt.Errorf("a --dependencies or --data-dependencies is required")
}
