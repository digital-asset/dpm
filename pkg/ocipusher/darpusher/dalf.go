package darpusher

import (
	"archive/zip"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/samber/lo"
)

func GetMainDalfHash(darPath string) (string, error) {
	manifest, err := readDar(darPath)
	if err != nil {
		return "", err
	}

	hash := extractHash(manifest)
	if hash == "" {
		return "", fmt.Errorf("could not extract Main-Dalf hash from the dar's manifest")
	}
	return hash, nil
}

// readDar extracts the manifest out of a dar
func readDar(darPath string) (string, error) {
	reader, err := zip.OpenReader(darPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	for _, f := range reader.File {
		if strings.EqualFold(f.Name, "META-INF/MANIFEST.MF") {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer func() { _ = rc.Close() }()

			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			return string(data), nil
		}
	}

	return "", fmt.Errorf("invalid dar: META-INF/MANIFEST.MF not found")
}

func extractHash(manifest string) string {
	re := regexp.MustCompile(`(?m)^Main-Dalf:[\s\S]*?\.dalf`)
	raw := re.FindString(manifest)
	if raw == "" {
		return ""
	}
	mainDalf := regexp.MustCompile(`\s+`).ReplaceAllString(raw, "")
	hash, _ := lo.Last(strings.Split(mainDalf, "-"))
	return strings.TrimSuffix(hash, ".dalf")
}
