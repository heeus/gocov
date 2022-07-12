package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"

	"github.com/heeus/htest/scanner"
	"github.com/heeus/htest/shared"
	"github.com/heeus/htest/shared/vos"
	"github.com/heeus/htest/tester"
)

func main() {
	// notest
	env := vos.Os()

	enforceFlag := flag.Bool("e", false, "Enforce 100% code coverage")
	verboseFlag := flag.Bool("v", false, "Verbose output")
	shortFlag := flag.Bool("short", false, "Pass the short flag to the go test command")
	timeoutFlag := flag.String("timeout", "", "Pass the timeout flag to the go test command")
	outputFlag := flag.String("o", "", "Override coverage file location")
	argsFlag := new(argsValue)
	flag.Var(argsFlag, "t", "Argument to pass to the 'go test' command. Can be used more than once.")
	loadFlag := flag.String("l", "", "Load coverage file(s) instead of running 'go test'")

	flag.Parse()

	setup := &shared.Setup{
		Env:      env,
		Paths:    shared.NewCache(env),
		Enforce:  *enforceFlag,
		Verbose:  *verboseFlag,
		Short:    *shortFlag,
		Timeout:  *timeoutFlag,
		Output:   *outputFlag,
		TestArgs: argsFlag.args,
		Load:     *loadFlag,
	}

	if err := Run(setup); err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}

	out := "coverage.out"
	printNotCoverLinks(setup, out)
	printTotalCoverage(setup, out)

}

func printNotCoverLinks(setup *shared.Setup, out string) {
	by, err := ioutil.ReadFile(out)
	if err != nil {
		fmt.Printf("Error reading coverage output file %s", out)
		os.Exit(1)
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
							filename = getFuleNameFromFullName(fullfilename)
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
	if len(pritnstsr) > 0 {
		fmt.Println("The following lines are not tested:")
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
func getFuleNameFromFullName(fullfilename string) string {
	if len(fullfilename) < 5 {
		return ""
	}
	strs := strings.Split(fullfilename, "/")
	if len(strs) < 3 {
		return ""
	}
	res := ""
	for i := 2; i < len(strs); i++ {
		if len(res) == 0 {
			res = strs[i]
		} else {
			res = res + "/" + strs[i]
		}
	}
	return res
}

func printTotalCoverage(setup *shared.Setup, out string) {
	currentDir, _ := setup.Env.Getwd()

	stdout := bytes.NewBufferString("")
	stderr := bytes.NewBufferString("")
	exe := exec.Command("go", "tool", "cover", "-func", out)
	exe.Dir = currentDir
	exe.Env = setup.Env.Environ()
	exe.Stdout = stdout
	exe.Stderr = stderr
	err := exe.Run()
	if err != nil {
		fmt.Printf("%v not found.", out)
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
	if setup.Load == "" {
		if err := t.Test(); err != nil {
			return errors.Wrapf(err, "Test")
		}
	} else {
		if err := t.Load(); err != nil {
			return errors.Wrapf(err, "Load")
		}
	}
	if err := t.ProcessExcludes(s.Excludes); err != nil {
		return errors.Wrapf(err, "ProcessExcludes")
	}
	if err := t.Save(); err != nil {
		return errors.Wrapf(err, "Save")
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