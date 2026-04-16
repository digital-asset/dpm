// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fileinfo

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"time"

	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/utils"
)

var (
	LegacyFileModeAnnotation = ociconsts.LegacyDpmAnnotation("file-mode")
	LegacyModTimeAnnotation  = ociconsts.LegacyDpmAnnotation("file-modtime")
	LegacyFileNameAnnotation = ociconsts.LegacyDpmAnnotation("file-name")

	FileModeAnnotation = ociconsts.DpmAnnotation("file-mode")
	ModTimeAnnotation  = ociconsts.DpmAnnotation("file-modtime")
	FileNameAnnotation = ociconsts.DpmAnnotation("file-name")
)

func missingAnnotations(a, b string) error {
	return fmt.Errorf("missing (%q, or %q) annotation", a, b)
}

type FileInfo struct {
	FileMode os.FileMode
	ModTime  time.Time
	FileName string
}

func (fi *FileInfo) AsAnnotations() map[string]string {
	mode := fmt.Sprintf("%o", fi.FileMode)
	modTime := fi.ModTime.Format(time.RFC3339)
	fileName := fi.FileName

	m := map[string]string{
		FileModeAnnotation: mode,
		ModTimeAnnotation:  modTime,
		FileNameAnnotation: fileName,
	}

	if os.Getenv(ociconsts.SkipLegacyOciAnnotationsEnvVar) != "true" {
		legacy := map[string]string{
			LegacyFileModeAnnotation: mode,
			LegacyModTimeAnnotation:  modTime,
			LegacyFileNameAnnotation: fileName,
		}
		maps.Copy(m, legacy)
	}

	return m
}

// Apply sets the fileinfo for the corresponding file on the filesystem
func (fi *FileInfo) Apply(rootPath string) error {
	filePath := filepath.Join(rootPath, fi.FileName)
	if err := os.Chmod(filePath, fi.FileMode); err != nil {
		return err
	}
	return os.Chtimes(filePath, fi.ModTime, fi.ModTime)
}

func NewFromAnnotations(annotations map[string]string) (*FileInfo, error) {
	if annotations == nil {
		return nil, fmt.Errorf("missing fileinfo annotations")
	}

	fileModeStr, ok := utils.GetWithFallback(annotations, FileModeAnnotation, LegacyFileModeAnnotation)
	if !ok {
		return nil, missingAnnotations(FileModeAnnotation, LegacyFileModeAnnotation)
	}
	fileMode, err := strconv.ParseUint(fileModeStr, 8, 32)
	if err != nil {
		return nil, err
	}

	modTimeStr, ok := utils.GetWithFallback(annotations, ModTimeAnnotation, LegacyModTimeAnnotation)
	if !ok {
		return nil, missingAnnotations(ModTimeAnnotation, LegacyModTimeAnnotation)
	}
	modTime, err := time.Parse(time.RFC3339, modTimeStr)
	if err != nil {
		return nil, err
	}

	fileName, ok := utils.GetWithFallback(annotations, FileNameAnnotation, LegacyFileNameAnnotation)
	if !ok {
		return nil, missingAnnotations(FileNameAnnotation, LegacyFileNameAnnotation)
	}

	return &FileInfo{
		FileMode: os.FileMode(fileMode),
		ModTime:  modTime,
		FileName: fileName,
	}, nil
}

func New(info os.FileInfo) *FileInfo {
	return &FileInfo{
		FileMode: info.Mode(),
		ModTime:  info.ModTime(),
		FileName: info.Name(),
	}
}
