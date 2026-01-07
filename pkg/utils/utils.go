// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// BoolEnvVar parses an env var as bool. Defaults to false
func BoolEnvVar(key string) (val bool, ok bool, err error) {
	var valStr string
	valStr, ok = os.LookupEnv(key)
	if !ok {
		return false, ok, nil
	}
	b, err := strconv.ParseBool(valStr)
	if err != nil {
		return false, ok, fmt.Errorf("invalid value for '%s' env var. Must be one of ('true', 'false')", key)
	}
	return b, ok, nil
}

var envVarRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func IsValidEnvVarIdentifier(key string) bool {
	return envVarRegex.MatchString(key)
}
