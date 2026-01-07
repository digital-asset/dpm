// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fileinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	ociconsts "daml.com/x/assistant/pkg/oci"
)

var (
	FileModeAnnotation = ociconsts.DAAnnotation("file-mode")
	ModTimeAnnotation  = ociconsts.DAAnnotation("file-modtime")
	FileNameAnnotation = ociconsts.DAAnnotation("file-name")
)

func missingAnnotation(a string) error {
	return fmt.Errorf("missing %s annotation", a)
}

type FileInfo struct {
	FileMode os.FileMode
	ModTime  time.Time
	FileName string
}

func (fi *FileInfo) AsAnnotations() map[string]string {
	return map[string]string{
		FileModeAnnotation: fmt.Sprintf("%o", fi.FileMode),
		ModTimeAnnotation:  fi.ModTime.Format(time.RFC3339),
		FileNameAnnotation: fi.FileName,
	}
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

	fileModeStr, ok := annotations[FileModeAnnotation]
	if !ok {
		return nil, missingAnnotation(FileModeAnnotation)
	}
	fileMode, err := strconv.ParseUint(fileModeStr, 8, 32)
	if err != nil {
		return nil, err
	}

	modTimeStr, ok := annotations[ModTimeAnnotation]
	if !ok {
		return nil, missingAnnotation(FileModeAnnotation)
	}
	modTime, err := time.Parse(time.RFC3339, modTimeStr)
	if err != nil {
		return nil, err
	}

	fileName, ok := annotations[FileNameAnnotation]
	if !ok {
		return nil, missingAnnotation(FileNameAnnotation)
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
