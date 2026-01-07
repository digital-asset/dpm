// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantconfig

const envVarPrefix = "DPM_"

const (
	// DpmHomeEnvVar
	// DPM_HOME is the absolute path to the `dpm` home directory
	DpmHomeEnvVar = envVarPrefix + "HOME"

	// DamlMultiPackageEnvVar
	// DPM_MULTI_PACKAGE is the absolute path to dir containing multi-package.yaml
	DamlMultiPackageEnvVar = envVarPrefix + "MULTI_PACKAGE"

	// OciRegistryEnvVar
	// DPM_REGISTRY overrides the OCI registry from which components and sdk-bundles are downloaded
	OciRegistryEnvVar = envVarPrefix + "REGISTRY"

	// RegistryAuthConfigPathEnvVar
	// DPM_REGISTRY_AUTH overrides the OCI registry auth file used
	// Contains a path to a config file similar to docker’s config.json, which will be used to authenticate to the configured registry
	// 	default: $HOME/.docker/config.json).
	RegistryAuthConfigPathEnvVar = envVarPrefix + "REGISTRY_AUTH"

	// AllowInsecureRegistryEnvVar
	// DPM_INSECURE_REGISTRY allows an insecure registry to be used (http instead of https, and without auth)
	AllowInsecureRegistryEnvVar = envVarPrefix + "INSECURE_REGISTRY"

	// LogLevelEnvVar
	// DPM_LOG_LEVEL sets the log level for the assistant.
	// 	Default: info
	//  Possible values: info error warning fatal debug
	LogLevelEnvVar = envVarPrefix + "LOG_LEVEL"

	// AutoInstallEnvVar
	// DPM_AUTO_INSTALL disables automatically installing the sdk-version specified in daml.yaml.
	// It also disables automatic installation of any remote components that are missing
	AutoInstallEnvVar = envVarPrefix + "AUTO_INSTALL"

	// EditionEnvVar
	// DPM_EDITION sets the edition of the assistant.
	// 	Possible values: enterprise private open-source
	EditionEnvVar = envVarPrefix + "EDITION"

	// DamlProjectEnvVar (deprecated)
	// DAML_PROJECT	is a path to a daml project directory.
	// This allows running a command in a project without changing directory
	DamlProjectEnvVar = "DAML_PROJECT"

	// DamlPackageEnvVar
	// DAML_PACKAGE	is a path to a daml package directory.
	// This allows running a command in a package context without changing directory
	DamlPackageEnvVar = "DAML_PACKAGE"

	// ResolutionFilePathEnvVar
	// Allows overriding the output file path for deep resolution
	ResolutionFilePathEnvVar = envVarPrefix + "RESOLUTION_FILE"

	// DpmSdkVersionEnvVar
	// Allows overriding the SDK version being used.
	// It's a global override that overrides the sdk version specified in any and all daml.yaml(s).
	// It also overrides the SDK version used outside package or multi-package context.
	// (It doesn't affect the `install` command(s))
	DpmSdkVersionEnvVar = "DPM_SDK_VERSION"
)
