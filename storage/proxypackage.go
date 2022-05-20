package storage

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/elastic/package-registry/packages"
)

type proxyPackageFileSystem struct {
	memory afero.MemMapFs
}

var _ packages.PackageFileSystem = new(proxyPackageFileSystem)

func newProxyPackageFileSystem(pi packageIndex) (*proxyPackageFileSystem, error) {
	var proxy proxyPackageFileSystem

	manifestFile, err := proxy.memory.Create("manifest.yml")
	if err != nil {
		return nil, errors.Wrap(err, "can't create virtual manifest file")
	}
	_, err = io.Copy(manifestFile, bytes.NewBuffer(pi.PackageManifest))
	if err != nil {
		return nil, errors.Wrap(err, "can't create virtual package manifest file")
	}
	err = manifestFile.Close()
	if err != nil {
		return nil, errors.Wrap(err, "can't close the virtual package manifest file")
	}

	for _, dsm := range pi.DataStreamManifests {
		var dataStreamManifest packages.DataStream
		err = json.Unmarshal(dsm, &dataStreamManifest)
		if err != nil {
			return nil, errors.Wrap(err, "can't unmarshal the virtual data stream manifest file")
		}

		dataStreamDir := filepath.Dir("data_stream", dataStreamManifest.)

		err = proxy.memory.MkdirAll(asset, 0755)
		if err != nil {
			return nil, errors.Wrapf(err, "proxy.memory.MkdirAll failed (path: %s)", dataStreamDir)
		}


	}

	for _, asset := range pi.Assets {
		if asset == "manifest.yml" {
			manifestFile, err := proxy.memory.Create("manifest.yml")
			if err != nil {
				return nil, errors.Wrap(err, "can't create virtual manifest file")
			}
			_, err = io.Copy(manifestFile, bytes.NewBuffer(pi.PackageManifest))
			if err != nil {
				return nil, errors.Wrap(err, "can't create virtual package manifest file")
			}
			err = manifestFile.Close()
			if err != nil {
				return nil, errors.Wrap(err, "can't close the virtual package manifest file")
			}
			continue
		}

		if strings.HasPrefix("data_stream/", asset) && strings.HasSuffix("/manifest.yml", asset) {
			dataStreamDir := filepath.Dir(asset)
			err := proxy.memory.MkdirAll(asset, 0755)
			if err != nil {
				return nil, errors.Wrapf(err, "proxy.memory.MkdirAll failed (path: %s)", dataStreamDir)
			}

			dataSteamManifestPath := filepath.Join(dataStreamDir, "manifest.yml")
			manifestFile, err := proxy.memory.Create(dataSteamManifestPath)
			if err != nil {
				return nil, errors.Wrap(err, "can't create virtual manifest file")
			}
			_, err = io.Copy(manifestFile, bytes.NewBuffer(pi.PackageManifest))
			if err != nil {
				return nil, errors.Wrap(err, "can't create virtual package manifest file")
			}
			err = manifestFile.Close()
			if err != nil {
				return nil, errors.Wrap(err, "can't close the virtual package manifest file")
			}
		}
	}

	return &proxy, nil
}

func (fs *proxyPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	return fs.memory.Stat(name)
}

func (fs *proxyPackageFileSystem) Open(name string) (packages.PackageFile, error) {
	return fs.memory.Open(name)
}

func (fs *proxyPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	return afero.Glob(&fs.memory, pattern)
}

func (fs *proxyPackageFileSystem) Close() error {
	return nil // nothing to close here
}
