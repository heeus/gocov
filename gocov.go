package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"

	"github.com/heeus/gocov/scanner"
	"github.com/heeus/gocov/shared"
	"github.com/heeus/gocov/shared/vos"
	"github.com/heeus/gocov/tester"
)

const (
	gomod = "go.mod"
)

func main() {
	// notest
	env := vos.Os()

	var enforceFlag bool
	var verboseFlag bool
	var shortFlag bool
	var timeoutFlag string
	var outputFlag string
	var loadFlag string

	argsFlag := new(argsValue)
	flag.Var(argsFlag, "t", "Argument to pass to the 'go test' command. Can be used more than once.")

	fs := flag.NewFlagSet("gocov", flag.ContinueOnError)
	fs.BoolVar(&enforceFlag, "e", false, "Enforce 100% code coverage")
	fs.BoolVar(&verboseFlag, "v", false, "Verbose output")
	fs.BoolVar(&shortFlag, "short", false, "Pass the short flag to the go test command")
	fs.StringVar(&timeoutFlag, "timeout", "", "Pass the timeout flag to the go test command")
	fs.StringVar(&outputFlag, "o", "", "Override coverage file location")
	fs.StringVar(&loadFlag, "l", "", "Load coverage file(s) instead of running 'go test'")
	start := 1
	notestParam := false
	notestdeptParam := false
	if len(os.Args) > 1 {
		notestParam = os.Args[1] == "notest"
		notestdeptParam = os.Args[1] == "notestdept"
		start = 2
	}
	fs.Parse(os.Args[start:])

	setup := &shared.Setup{
		Env:        env,
		Paths:      shared.NewCache(env),
		Enforce:    enforceFlag,
		Verbose:    verboseFlag,
		Short:      shortFlag,
		Timeout:    timeoutFlag,
		Output:     outputFlag,
		Notest:     notestParam,
		Notestdept: notestdeptParam,
		TestArgs:   argsFlag.args,
		Load:       loadFlag,
	}

	if err := Run(setup); err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}

	out := tester.CoverageFileName
	outun := tester.UncoverageFileName
	if setup.Notest || setup.Notestdept {
		printNotCoverLinks(setup, outun, false)
	} else {
		printNotCoverLinks(setup, out, true)
		printTotalCoverage(setup, out)
	}
	os.Remove(out)
	os.Remove(outun)
}

func printNotCoverLinks(setup *shared.Setup, fn string, covered bool) {
	by, err := ioutil.ReadFile(fn)
	if err != nil {
		return
	}
	lines := strings.Split(string(by), "\n")
	var posfrom string
	var filename string

	var pritnstsr []string
	for i, str := range lines {
		if i > 0 {
			strline := strings.Split(str, " ")
			if len(strline) == 3 {
				posline := strings.Split(str, ",")
				if len(posline) == 2 {
					poslinefrom := strings.Split(posline[0], ":")
					posto := posline[1]
					if badStatus(posto) {
						if len(poslinefrom) == 2 {
							posfrom = poslinefrom[1]
							fullfilename := poslinefrom[0]
							if setup.Notest || setup.Notestdept {
								filename = getFullNameFromPath(fullfilename)
							} else {
								filename = getFullNameFromCover(fullfilename)
							}
						}
						if len(posfrom) > 0 && len(filename) > 0 {
							posfrom = strings.ReplaceAll(posfrom, ".", ":")
							pritnstsr = append(pritnstsr, filename+":"+posfrom)
						}
					}
				}
			}
		}
	}
	var s string
	if covered {
		s = "------------------------------------\t\n" +
			"The following lines are not tested:\t\n" +
			"------------------------------------"
	} else {
		flag := ""
		if setup.Notest {
			flag = "'notest'"
		} else {
			flag = "'notestdept'"

		}
		s = "-------------------------------------------------\t\n" +
			"The following lines have instruction " + flag + ":\t\n" +
			"-------------------------------------------------"
	}
	if len(pritnstsr) > 0 {
		fmt.Println(s)
		for _, str := range pritnstsr {
			fmt.Println(str)
		}
	}
}

func badStatus(statusline string) bool {
	if len(statusline) < 3 {
		return true
	}
	strs := strings.Split(statusline, " ")
	if len(strs) < 3 {
		return true
	}
	return strs[2] == "0"
}
func getFullNameFromPath(fullfilename string) string {
	mydir, _ := os.Getwd()
	start := len(mydir) + 1
	return "./" + substr(fullfilename, start, len(fullfilename)-start)
}

func getFullNameFromCover(fullfilename string) string {

	// Search first go.mod in current and parent folders
	goModfile, path := findGoMod()
	fb, err := ioutil.ReadFile(goModfile)
	if err != nil {
		return ""
	}
	f, err := modfile.Parse(goModfile, fb, nil)
	if err != nil {
		return ""
	}

	pos := strings.Index(fullfilename, f.Module.Mod.Path)
	if pos < 0 {
		return ""
	}
	cutpath := f.Module.Mod.Path + strings.ReplaceAll(path, "\\", "/")
	return "./" + substr(fullfilename, len(cutpath)+1, len(fullfilename))

}

func findGoMod() (goModPath string, addPath string) {
	addPath = ""
	root, _ := os.Getwd()
	pattern := gomod
	for {
		matches, err := filepath.Glob(root + "/" + pattern)

		if err != nil {
			fmt.Println(err)
		}

		if len(matches) != 0 {
			return matches[0], addPath
		}
		prevroot := root
		root = filepath.Dir(root)
		if prevroot == root {
			break
		}
		if root == "" {
			break
		}
		pos := strings.Index(prevroot, root)
		addPath = substr(prevroot, len(root)-pos, len(prevroot)-len(root)+1)
	}
	return "", addPath
}

func substr(input string, start int, length int) string {
	asRunes := []rune(input)

	if start >= len(asRunes) {
		return ""
	}

	if start+length > len(asRunes) {
		length = len(asRunes) - start
	}

	return string(asRunes[start : start+length])
}

func printTotalCoverage(setup *shared.Setup, fn string) {
	currentDir, _ := setup.Env.Getwd()

	stdout := bytes.NewBufferString("")
	stderr := bytes.NewBufferString("")
	exe := exec.Command("go", "tool", "cover", "-func", fn)
	exe.Dir = currentDir
	exe.Env = setup.Env.Environ()
	exe.Stdout = stdout
	exe.Stderr = stderr
	err := exe.Run()
	if err != nil {
		fmt.Printf("%v not found.", fn)
		os.Exit(1)
	}

	str := stdout.String()
	strs := strings.Split(str, "\n")
	if len(strs) > 0 {
		s := strs[len(strs)-2]
		strt := strings.Split(s, "\t")
		if len(strt) > 0 {
			ps := strt[len(strt)-1]
			fmt.Printf("coverage: %s of statements", ps)
		}
	}
}

// Run initiates the command with the provided setup
func Run(setup *shared.Setup) error {
	if err := setup.Parse(flag.Args()); err != nil {
		return errors.Wrapf(err, "Parse")
	}

	s := scanner.New(setup)
	if err := s.LoadProgram(); err != nil {
		return errors.Wrapf(err, "LoadProgram")
	}
	if err := s.ScanPackages(); err != nil {
		return errors.Wrapf(err, "ScanPackages")
	}

	t := tester.New(setup)

	if !(setup.Notest || setup.Notestdept) {
		if setup.Load == "" {
			if err := t.Test(); err != nil {
				return errors.Wrapf(err, "Test")
			}
		} else {
			if err := t.Load(); err != nil {
				return errors.Wrapf(err, "Load")
			}
		}
	}
	if err := t.ProcessExcludes(s.Excludes); err != nil {
		return errors.Wrapf(err, "ProcessExcludes")
	}

	if !(setup.Notest || setup.Notestdept) {
		if err := t.Save(); err != nil {
			return errors.Wrapf(err, "Save")
		}
	}

	if setup.Notest {
		if err := t.SaveUn(shared.Notest); err != nil {
			return errors.Wrapf(err, "SaveUn")
		}
	}
	if setup.Notestdept {
		if err := t.SaveUn(shared.Notestdept); err != nil {
			return errors.Wrapf(err, "SaveUn")
		}
	}

	if err := t.Enforce(); err != nil {
		return errors.Wrapf(err, "Enforce")
	}

	return nil
}

type argsValue struct {
	args []string
}

var _ flag.Value = (*argsValue)(nil)

func (v *argsValue) String() string {
	// notest
	if v == nil {
		return ""
	}
	return strings.Join(v.args, " ")
}
func (v *argsValue) Set(s string) error {
	// notest
	v.args = append(v.args, s)
	return nil
}
