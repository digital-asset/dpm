// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolutionerrors

import "errors"

const (
	SdkNotInstalled   = "SDK_NOT_INSTALLED"
	MalformedDamlYaml = "MALFORMED_DAML_YAML"
	DamlYamlNotFound  = "DAML_YAML_NOT_FOUND"
	UnknownError      = "UNKNOWN_ERROR"
)

type ResolutionError struct {
	Code  string
	Cause error
}

func (r *ResolutionError) Error() string {
	if r.Cause != nil {
		return r.Code + ": " + r.Cause.Error()
	}
	return r.Code
}

func (r *ResolutionError) MarshalYAML() (interface{}, error) {
	var causeStr string
	if r.Cause != nil {
		causeStr = r.Cause.Error()
	}
	return map[string]interface{}{
		"code":  r.Code,
		"cause": causeStr,
	}, nil
}

func (r *ResolutionError) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var aux struct {
		Code  string `yaml:"code"`
		Cause string `yaml:"cause"`
	}
	if err := unmarshal(&aux); err != nil {
		return err
	}
	r.Code = aux.Code
	if aux.Cause != "" {
		r.Cause = errors.New(aux.Cause)
	}
	return nil
}

func (r *ResolutionError) Unwrap() error {
	return r.Cause
}

var _ error = (*ResolutionError)(nil)

func NewSdkNotInstalledError(cause error) *ResolutionError {
	return &ResolutionError{
		Code:  SdkNotInstalled,
		Cause: cause,
	}
}

func NewMalformedDamlYamlError(cause error) *ResolutionError {
	return &ResolutionError{
		Code:  MalformedDamlYaml,
		Cause: cause,
	}
}

func NewDamlYamlNotFoundError(cause error) *ResolutionError {
	return &ResolutionError{
		Code:  DamlYamlNotFound,
		Cause: cause,
	}
}

func NewUnknownError(cause error) *ResolutionError {
	return &ResolutionError{
		Code:  UnknownError,
		Cause: cause,
	}
}

func Standardize(err error) *ResolutionError {
	if err == nil {
		return nil
	}

	var resErr *ResolutionError
	if errors.As(err, &resErr) {
		return resErr
	}

	return NewUnknownError(err)
}
