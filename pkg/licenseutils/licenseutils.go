package licenseutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/lo"
)

const ComponentLicenseFilename = "LICENSE"
const TarballLicensesFilename = "LICENSES"

// WriteLicensesFile combines components licenses into single LICENSES file.
// licenses is map from component name to its LICENSE's file path
func WriteLicensesFile(licenses map[string]string, outputDir string) error {
	var b strings.Builder

	// Header
	b.WriteString("LICENSES\n")
	b.WriteString("============================================================\n\n")
	b.WriteString("This product includes multiple software components.\n")
	b.WriteString("The license terms for each component are provided below.")

	sortedComponents := lo.Keys(licenses)
	sort.Strings(sortedComponents)

	for _, comp := range sortedComponents {
		license, err := os.ReadFile(licenses[comp])
		if err != nil {
			return err
		}

		b.WriteString("\n\n------------------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("Component: %s\n\n", comp))
		b.WriteString(strings.TrimSpace(string(license)))
	}
	b.WriteString("\n")

	return os.WriteFile(filepath.Join(outputDir, TarballLicensesFilename), []byte(b.String()), 0644)
}
