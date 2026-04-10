// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantconfig

const (
	DamlMultiPackageFilename    = "multi-package.yaml"
	DamlPackageFilename         = "daml.yaml"
	DpmLockFileName             = "dpm.lock"
	DpmMultiPackageLockFileName = "multi-package.lock"
	DarManifestName             = "dar.yaml"
	DefaultOciRegistry          = "europe-docker.pkg.dev/da-images/public" // stable prod public registry as the default

	AssistantUserAgentPrefix = "dpm"

	DpmConfigFileName = "dpm-config.yaml"

	DpmPathInjectedEnvVar = "DPM_BIN_PATH"

	// BlankSdkVersion this will be the value of the DPM_SDK_VERSION env var that dpm injects into the commands it runs
	// in the blank (aka no-sdk) case.
	BlankSdkVersion = ""
)
