package tester

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/heeus/gocov/shared"
	"github.com/heeus/gocov/tester/logger"
	"github.com/heeus/gocov/tester/merge"
	"github.com/pkg/errors"
	"golang.org/x/tools/cover"
)

const (
	CoverageFileName   = "coverage.out"
	UncoverageFileName = "uncoverage.out"
)

// New creates a new Tester with the provided setup
func New(setup *shared.Setup) *Tester {
	t := &Tester{
		setup: setup,
	}
	return t
}

// Tester runs tests and merges coverage files
type Tester struct {
	setup              *shared.Setup
	cover              string
	Results            []*cover.Profile
	NocoverResults     []*cover.Profile
	NocoverdeptResults []*cover.Profile
	NocoverautoResults []*cover.Profile
}

// Load loads pre-prepared coverage files instead of running 'go test'
func (t *Tester) Load() error {
	files, err := filepath.Glob(t.setup.Load)
	if err != nil {
		return errors.Wrap(err, "Error loading coverage files")
	}
	for _, fpath := range files {
		if err := t.processCoverageFile(fpath); err != nil {
			return err
		}
	}
	return nil
}

// Test initiates the tests and merges the coverage files
func (t *Tester) Test() error {

	var err error
	if t.cover, err = ioutil.TempDir("", "coverage"); err != nil {
		return errors.Wrap(err, "Error creating temporary coverage dir")
	}
	defer os.RemoveAll(t.cover)

	for _, spec := range t.setup.Packages {
		if err := t.processDir(spec.Dir); err != nil {
			return err
		}
	}

	return nil
}

func (t *Tester) doSave(extype shared.ExcludeType, outf string) error {
	var rs []*cover.Profile

	switch {
	case extype == shared.Nocover:
		rs = t.NocoverResults
	case extype == shared.Nocoverdept:
		rs = t.NocoverdeptResults
	case extype == shared.Nocoverauto:
		rs = t.NocoverautoResults
	default:
		rs = t.Results
	}

	if len(rs) == 0 {
		fmt.Fprintln(t.setup.Env.Stdout(), "No results")
		return nil
	}
	currentDir, err := t.setup.Env.Getwd()
	if err != nil {
		return errors.Wrap(err, "Error getting working dir")
	}
	out := filepath.Join(currentDir, outf)
	if t.setup.Output != "" {
		out = t.setup.Output
	}
	f, err := os.Create(out)
	if err != nil {
		return errors.Wrapf(err, "Error creating output coverage file %s", out)
	}
	defer f.Close()

	if extype == shared.Nocoverauto {
		fmt.Println("Auto excluded blocks")
		fmt.Println("______________________")
	}
	if extype == shared.Nocoverdept {
		fmt.Println("//nocoverdept excluded blocks")
		fmt.Println("______________________")
	}
	if extype == shared.Nocover {
		fmt.Println("//nocover excluded blocks")
		fmt.Println("______________________")
	}
	merge.DumpProfiles(rs, f)
	return nil
}

// Save saves the coverage file
func (t *Tester) Save() error {
	return t.doSave(shared.Nocoverall, CoverageFileName)
}

// SaveUn saves the uncoverage file
func (t *Tester) SaveUn(extype shared.ExcludeType) error {
	return t.doSave(extype, UncoverageFileName)
}

// Enforce returns an error if code is untested if the -e command line option
// is set
func (t *Tester) Enforce() error {
	if !t.setup.Enforce {
		return nil
	}
	untested := make(map[string][]cover.ProfileBlock)
	for _, r := range t.Results {
		for _, b := range r.Blocks {
			if b.Count == 0 {
				if len(untested[r.FileName]) > 0 {
					// check if the new block is directly after the last one
					last := untested[r.FileName][len(untested[r.FileName])-1]
					if b.StartLine <= last.EndLine+1 {
						last.EndLine = b.EndLine
						last.EndCol = b.EndCol
						untested[r.FileName][len(untested[r.FileName])-1] = last
						continue
					}
				}
				untested[r.FileName] = append(untested[r.FileName], b)
			}
		}
	}

	if len(untested) == 0 {
		return nil
	}

	var s string
	for name, blocks := range untested {
		fpath, err := t.setup.Paths.FilePath(name)
		if err != nil {
			return err
		}
		by, err := ioutil.ReadFile(fpath)
		if err != nil {
			return errors.Wrapf(err, "Error reading source file %s", fpath)
		}
		lines := strings.Split(string(by), "\n")
		for _, b := range blocks {
			s += fmt.Sprintf("%s:%d-%d:\n", name, b.StartLine, b.EndLine)
			undented := undent(lines[b.StartLine-1 : b.EndLine])
			s += strings.Join(undented, "\n")
		}
	}
	return errors.Errorf("Error - untested code:\n%s", s)

}

// ProcessExcludes uses the output from the scanner package and removes blocks
// from the merged coverage file.
func (t *Tester) ProcessExcludes(excludes map[string]map[int]shared.ExcludeType) error {
	var nocoverexclud []*cover.Profile
	var nocoverdeptexclud []*cover.Profile
	var nocoverautoexclud []*cover.Profile
	var processed []*cover.Profile

	var p *cover.Profile
	for _, p = range t.Results {

		// Filenames in t.Results are in go package form. We need to convert to
		// filepaths before use
		fpath, err := t.setup.Paths.FilePath(p.FileName)
		if err != nil {
			return err
		}

		f, ok := excludes[fpath]
		if !ok {
			// no excludes in this file - add the profile unchanged
			processed = append(processed, p)
			continue
		}
		var blocks []cover.ProfileBlock
		var nocoverblocks []cover.ProfileBlock
		var nocoverdeptblocks []cover.ProfileBlock
		var nocoverautoblocks []cover.ProfileBlock
		for _, b := range p.Blocks {
			excluded := shared.Nocoverall
			for line := b.StartLine; line <= b.EndLine; line++ {
				if ex, ok := f[line]; ok {
					if ex == shared.Nocover {
						excluded = shared.Nocover
						break
					} else if ex == shared.Nocoverdept {
						excluded = shared.Nocoverdept
						break
					} else if ex == shared.Nocoverauto {
						excluded = shared.Nocoverauto
						break
					}
				}
			}
			if excluded == shared.Nocoverall || b.Count > 0 {
				// include blocks that are not excluded
				// also include any blocks that have coverage
				blocks = append(blocks, b)
			} else {
				switch {
				case excluded == shared.Nocover:
					nocoverblocks = append(nocoverblocks, b)
				case excluded == shared.Nocoverdept:
					nocoverdeptblocks = append(nocoverdeptblocks, b)
				case excluded == shared.Nocoverauto:
					nocoverautoblocks = append(nocoverautoblocks, b)
				}
			}
		}
		profile := &cover.Profile{
			FileName: p.FileName,
			Mode:     p.Mode,
			Blocks:   blocks,
		}
		nocoverprofile := &cover.Profile{
			FileName: p.FileName,
			Mode:     p.Mode,
			Blocks:   nocoverblocks,
		}
		nocoverdeptprofile := &cover.Profile{
			FileName: p.FileName,
			Mode:     p.Mode,
			Blocks:   nocoverdeptblocks,
		}
		nocoverautoprofile := &cover.Profile{
			FileName: p.FileName,
			Mode:     p.Mode,
			Blocks:   nocoverautoblocks,
		}
		processed = append(processed, profile)
		nocoverexclud = append(nocoverexclud, nocoverprofile)
		nocoverdeptexclud = append(nocoverdeptexclud, nocoverdeptprofile)
		nocoverautoexclud = append(nocoverautoexclud, nocoverautoprofile)
	}
	t.Results = processed
	t.NocoverResults = nocoverexclud
	t.NocoverdeptResults = nocoverdeptexclud
	t.NocoverautoResults = nocoverautoexclud
	return nil
}

func (t *Tester) processDir(dir string) error {

	coverfile := filepath.Join(
		t.cover,
		fmt.Sprintf("%x", md5.Sum([]byte(dir)))+".out",
	)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "Error reading files from %s", dir)
	}

	foundTest := false
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "_test.go") {
			foundTest = true
		}
	}
	if !foundTest {
		// nocover
		return nil
	}

	combined, _, stderr := logger.Log(
		t.setup.Verbose,
		t.setup.Env.Stdout(),
		t.setup.Env.Stderr(),
	)

	var args []string
	var pkgs []string
	for _, s := range t.setup.Packages {
		pkgs = append(pkgs, s.Path)
	}
	args = append(args, "test")
	if t.setup.Short {
		// nocover
		// TODO: add test
		args = append(args, "-short")
	}
	if t.setup.Timeout != "" {
		// nocover
		// TODO: add test
		args = append(args, "-timeout", t.setup.Timeout)
	}
	args = append(args, fmt.Sprintf("-coverpkg=%s", strings.Join(pkgs, ",")))
	args = append(args, fmt.Sprintf("-coverprofile=%s", coverfile))
	if t.setup.Verbose {
		args = append(args, "-v")
	}
	if len(t.setup.TestArgs) > 0 {
		// nocover
		args = append(args, t.setup.TestArgs...)
	}
	if t.setup.Verbose {
		fmt.Fprintf(
			t.setup.Env.Stdout(),
			"Running test: %s\n",
			strings.Join(append([]string{"go"}, args...), " "),
		)
	}

	exe := exec.Command("go", args...)
	exe.Dir = dir
	exe.Env = t.setup.Env.Environ()
	exe.Stdout = nil
	exe.Stderr = stderr
	err = exe.Run()
	if strings.Contains(combined.String(), "no buildable Go source files in") {
		// nocover
		return nil
	}
	if err != nil {
		// TODO: Remove when https://github.com/dave/courtney/issues/4 is fixed
		// nocover
		if t.setup.Verbose {
			// They will already have seen the output
			return errors.Wrap(err, "Error executing test")
		}
		return errors.Wrapf(err, "Error executing test \nOutput:[\n%s]\n", combined.String())
	}
	return t.processCoverageFile(coverfile)
}

func (t *Tester) processCoverageFile(filename string) error {
	profiles, err := cover.ParseProfiles(filename)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if t.Results, err = merge.AddProfile(t.Results, p); err != nil {
			return err
		}
	}
	return nil
}

func undent(lines []string) []string {

	indentRegex := regexp.MustCompile("[^\t]")
	mindent := -1

	for _, line := range lines {
		loc := indentRegex.FindStringIndex(line)
		if len(loc) == 0 {
			// nocover
			// string is empty?
			continue
		}
		if mindent == -1 || loc[0] < mindent {
			mindent = loc[0]
		}
	}

	var out []string
	for _, line := range lines {
		if line == "" {
			// nocover
			out = append(out, "")
		} else {
			out = append(out, "\t"+line[mindent:])
		}
	}
	return out
}
