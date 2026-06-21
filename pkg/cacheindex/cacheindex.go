package cacheindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/utils"
	"github.com/opencontainers/go-digest"
)

const CacheIndexSchema = "cache-index/v1"

type CacheIndex struct {
	AbsolutePath string
}

type CacheIndexContents struct {
	Schema     string                         `json:"schema"`
	Components map[string]CacheIndexComponent `json:"components"`
}

type CacheIndexComponent struct {
	Version string `json:"version"`

	// this is not needed but is being included because having it will allow us
	// to make bold changes if we want to better flesh out / overhaul the concept of "name"
	// in dpm in the future
	Name string `json:"name"`
}

func (c *CacheIndex) Store(d digest.Digest, name, version string) error {
	if err := d.Validate(); err != nil {
		return err
	}

	contents, err := c.read()
	if err != nil {
		return err
	}

	contents.Components[d.String()] = CacheIndexComponent{
		Name:    name,
		Version: version,
	}

	return c.write(contents)
}

// Init writes an empty cache index if one doesn't already exist.
func (c *CacheIndex) Init() error {
	if _, err := os.Stat(c.AbsolutePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := utils.EnsureDirs(filepath.Dir(c.AbsolutePath)); err != nil {
		return err
	}

	contents := &CacheIndexContents{
		Schema:     CacheIndexSchema,
		Components: map[string]CacheIndexComponent{},
	}

	return c.write(contents)
}

func (c *CacheIndex) Get(digest string) (name string, version string, ok bool, err error) {
	if !strings.HasPrefix(digest, "sha256:") {
		return "", "", false, fmt.Errorf("the digest to read from the cache index must be a sha256 digest with the 'sha256:' prefix")
	}

	if err := c.Init(); err != nil {
		return "", "", false, err
	}

	contents, err := c.read()
	if err != nil {
		return "", "", false, err
	}

	component, ok := contents.Components[digest]
	if !ok {
		return "", "", false, nil
	}

	return component.Name, component.Version, true, nil
}

// read assumes the index file already exists.
func (c *CacheIndex) read() (*CacheIndexContents, error) {
	b, err := os.ReadFile(c.AbsolutePath)
	if err != nil {
		return nil, err
	}

	var contents CacheIndexContents
	if err := json.Unmarshal(b, &contents); err != nil {
		return nil, err
	}

	if contents.Components == nil {
		contents.Components = map[string]CacheIndexComponent{}
	}

	if contents.Schema == "" {
		contents.Schema = CacheIndexSchema
	}

	return &contents, nil
}

func (c *CacheIndex) write(contents *CacheIndexContents) error {
	if contents.Schema == "" {
		contents.Schema = CacheIndexSchema
	}

	if contents.Components == nil {
		contents.Components = map[string]CacheIndexComponent{}
	}

	b, err := json.Marshal(contents)
	if err != nil {
		return err
	}

	return os.WriteFile(c.AbsolutePath, b, 0o644)
}
