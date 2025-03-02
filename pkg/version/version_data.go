package version

import (
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Callback a callback function for processing version information. Return true to continue processing
// or false to terminate the loop
type Callback func(kind VersionKind, name string, version *StableVersion) (bool, error)

// VersionKind represents the kind of version
type VersionKind string

const (
	// KindChart represents a chart version
	KindChart VersionKind = "charts"

	// KindPackage represents a package version
	KindPackage VersionKind = "packages"
)

var (
	// Kinds all the version kinds
	Kinds = []VersionKind{
		KindChart,
		KindPackage,
	}
	// KindStrings all the kinds as strings for validating CLI arguments
	KindStrings = []string{
		string(KindChart),
		string(KindPackage),
	}
)

// StableVersion stores the stable version information
type StableVersion struct {
	Version string `yaml:"version,omitempty"`
	GitURL  string `yaml:"gitUrl,omitempty"`
	URL     string `yaml:"url,omitempty"`
}

// LoadStableVersion loads the stable version data from the version configuration directory returning an empty object if there is
// no specific stable version configuration available
func LoadStableVersion(wrkDir string, kind VersionKind, name string) (*StableVersion, error) {
	path := filepath.Join(wrkDir, string(kind), name+".yml")
	return LoadStableVersionFile(path)
}

// LoadStableVersionFile loads the stable version data from the given file name
func LoadStableVersionFile(path string) (*StableVersion, error) {
	version := &StableVersion{}
	exists, err := util.FileExists(path)
	if err != nil {
		return version, errors.Wrapf(err, "failed to check if file exists %s", path)
	}
	if !exists {
		return version, nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return version, errors.Wrapf(err, "failed to load YAML file %s", path)
	}
	err = yaml.Unmarshal(data, version)
	if err != nil {
		return version, errors.Wrapf(err, "failed to unmarshal YAML for file %s", path)
	}
	return version, err
}

// LoadStableVersionNumber loads just the stable version number for the given kind and name
func LoadStableVersionNumber(wrkDir string, kind VersionKind, name string) (string, error) {
	data, err := LoadStableVersion(wrkDir, kind, name)
	if err != nil {
		return "", err
	}
	version := data.Version
	if version != "" {
		log.Infof("using stable version %s from %s of %s from %s\n", util.ColorInfo(version), string(kind), util.ColorInfo(name), wrkDir)
	} else {
		log.Warnf("could not find a stable version from %s of %s from %s\nFor background see: https://jenkins-x.io/architecture/version-stream/\n", string(kind), name, wrkDir)
		log.Infof("Please lock this version down via the command: 'jx step create version pr -k %s -n %s'\n", string(kind), name)
	}
	return version, err
}

// SaveStableVersion saves the version file
func SaveStableVersion(wrkDir string, kind VersionKind, name string, version *StableVersion) error {
	data, err := yaml.Marshal(version)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal data to YAML %#v", version)
	}
	path := filepath.Join(wrkDir, string(kind), name+".yml")
	dir, _ := filepath.Split(path)
	err = os.MkdirAll(dir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	err = ioutil.WriteFile(path, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", path)
	}
	return nil
}

// ForEachVersion processes all of the versions in the wrkDir using the given callback function.
func ForEachVersion(wrkDir string, callback Callback) error {
	for _, kind := range Kinds {
		err := ForEachKindVersion(wrkDir, kind, callback)
		if err != nil {
			return err
		}
	}
	return nil
}

// ForEachKindVersion processes all of the versions in the wrkDir and kind using the given callback function.
func ForEachKindVersion(wrkDir string, kind VersionKind, callback Callback) error {
	kindString := string(kind)
	kindDir := filepath.Join(wrkDir, kindString)
	glob := filepath.Join(kindDir, "*", "*.yml")
	paths, err := filepath.Glob(glob)
	if err != nil {
		return errors.Wrapf(err, "failed to find glob: %s", glob)
	}
	for _, path := range paths {
		versionData, err := LoadStableVersionFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to load VersionData for file: %s", path)
		}
		name, err := filepath.Rel(kindDir, path)
		if err != nil {
			return errors.Wrapf(err, "failed to extract base path from %s", path)
		}
		ext := filepath.Ext(name)
		if ext != "" {
			name = strings.TrimSuffix(name, ext)
		}
		ok, err := callback(kind, name, versionData)
		if err != nil {
			return errors.Wrapf(err, "failed to process kind %s name %s", kindString, name)
		}
		if !ok {
			break
		}
	}
	return nil
}
