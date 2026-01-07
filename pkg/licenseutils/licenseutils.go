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
func WriteLicensesFile(licenses map[string][]byte, outputDir string) error {
	var b strings.Builder

	// Header
	fmt.Fprintln(&b, "LICENSES")
	fmt.Fprintln(&b, "============================================================")
	fmt.Fprintln(&b, "")
	fmt.Fprintln(&b, "This product includes multiple software components.")
	fmt.Fprint(&b, "The license terms for each component are provided below.")

	sortedComponents := lo.Keys(licenses)
	sort.Strings(sortedComponents)

	for _, comp := range sortedComponents {
		license := licenses[comp]
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "------------------------------------------------------------")
		fmt.Fprintf(&b, "Component: %s\n\n", comp)
		b.WriteString(strings.TrimSuffix(strings.TrimPrefix(string(license), "\n"), "\n"))
	}
	fmt.Fprintln(&b, "")

	return os.WriteFile(filepath.Join(outputDir, TarballLicensesFilename), []byte(b.String()), 0644)
}
