// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantversion

// To be populated at build-time, e.g.:
// go build -ldflags "-X 'daml.com/x/assistant/cmd/dpm/cmd/version.AssistantVersion=1.2.3'"
var (
	AssistantVersion string
	Build            string
	BuildDate        string
)

type VersionInfo struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	BuildDate string `json:"buildDate"`
}

func defaultUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func Get() VersionInfo {
	return VersionInfo{
		Version:   defaultUnknown(AssistantVersion),
		Build:     defaultUnknown(Build),
		BuildDate: defaultUnknown(Build),
	}
}

func GetAssistantVersion() string {
	return defaultUnknown(AssistantVersion)
}

func GetBuild() string {
	return defaultUnknown(Build)
}

func GetBuildDate() string {
	return defaultUnknown(BuildDate)
}
