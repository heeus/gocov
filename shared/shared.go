package shared

import (
	"errors"
	"strings"

	"github.com/heeus/gocov/shared/vos"
)

type ExcludeType byte

const (
	Notestall ExcludeType = iota
	Notest
	Notestdept
)

// Setup holds globals, environment and command line flags for the courtney
// command
type Setup struct {
	Env        vos.Env
	Paths      *Cache
	Enforce    bool
	Verbose    bool
	Short      bool
	Notest     bool
	Notestdept bool
	Timeout    string
	Load       string
	Output     string
	TestArgs   []string
	Packages   []PackageSpec
}

// PackageSpec identifies a package by dir and path
type PackageSpec struct {
	Dir  string
	Path string
}

// Parse parses a slice of strings into the Packages slice
func (s *Setup) Parse(args []string) error {

	if len(args) == 0 {
		args = []string{"./..."}
	}

	packages := map[string]string{}
	for _, ppath := range args {
		ppath = strings.TrimSuffix(ppath, "/")
		paths, err := s.Paths.Dirs(ppath)
		if err != nil {
			return errors.New("Package to test not found")
		}

		for importPath, dir := range paths {
			packages[importPath] = dir
		}
	}

	for ppath, dir := range packages {
		s.Packages = append(s.Packages, PackageSpec{Path: ppath, Dir: dir})
	}

	return nil
}
