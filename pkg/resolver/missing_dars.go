package resolver

import (
	"errors"
	"fmt"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
)

func isDarNotInstalled(err error) bool {
	var resErr *resolutionerrors.ResolutionError
	return errors.As(err, &resErr) && resErr.Code == resolutionerrors.DarNotInstalled
}

func formatMissingDarsError(missing []*damlpackage.ParsedDarDependency) error {
	if len(missing) == 0 {
		return nil
	}
	if len(missing) == 1 {
		return resolutionerrors.NewDarNotInstalled(fmt.Errorf(
			"%s is not installed. Run 'dpm install package' or 'dpm update'",
			missingDarLabel(missing[0]),
		))
	}

	type releaseGroup struct {
		baseLine string
		assets   []string
	}
	byRelease := map[string]*releaseGroup{}
	var other []*damlpackage.ParsedDarDependency

	for _, dep := range missing {
		if dep.Git.Release && strings.TrimSpace(dep.Git.DarPath) != "" {
			base := gitReleaseBaseLine(dep)
			g, ok := byRelease[base]
			if !ok {
				g = &releaseGroup{baseLine: base}
				byRelease[base] = g
			}
			g.assets = append(g.assets, dep.Git.DarPath)
			continue
		}
		other = append(other, dep)
	}

	if len(byRelease) == 1 && len(other) == 0 {
		var group *releaseGroup
		for _, g := range byRelease {
			group = g
		}
		if len(group.assets) > 1 {
			return resolutionerrors.NewDarNotInstalled(fmt.Errorf(
				"%d git release assets from %s are not installed. Run 'dpm install package' or 'dpm update' to download them (e.g. %q)",
				len(group.assets),
				group.baseLine,
				group.assets[0],
			))
		}
	}

	return resolutionerrors.NewDarNotInstalled(fmt.Errorf(
		"%d dars are not installed. Run 'dpm install package' or 'dpm update' (e.g. %q)",
		len(missing),
		missingDarLabel(missing[0]),
	))
}

func missingDarLabel(dep *damlpackage.ParsedDarDependency) string {
	if dep != nil && dep.Scheme() == "git" {
		return gitparse.FormatGitYamlLine(dep.Git)
	}
	if dep != nil && dep.FullUrl != nil {
		return dep.FullUrl.String()
	}
	return "dar"
}

func gitReleaseBaseLine(dep *damlpackage.ParsedDarDependency) string {
	return gitparse.FormatGitReleaseBaseLine(dep.Git)
}
