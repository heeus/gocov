package scanner_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"path/filepath"

	"github.com/heeus/gocov/scanner"
	"github.com/heeus/gocov/shared"
	"github.com/heeus/gocov/shared/builder"
	"github.com/heeus/gocov/shared/vos"
)


func TestFunctionExpressions(t *testing.T) {
	tests := map[string]string{
		"function expression params": `package foo
			
			func Baz() error { 
				var f func(int) error
				if f(4) != nil {   
					return f(5)
				}
				return nil
			}
			`,
		"function expression params 2": `package foo
			
			func Baz() error { 
				var f func(...int) error
				if f(4) != nil {   
					return f(4, 4)
				}
				return nil
			}
			`,
		"function expression elipsis": `package foo
			
			func Baz() error { 
				var f func(...interface{}) error
				var a []interface{}
				if f(a) != nil {   
					return f(a...)
				}
				return nil
			}
			`,
	}
	test(t, tests)
}

func TestComments(t *testing.T) {
	tests := map[string]string{
		"scope": `package foo
			
			func Baz() int { 
				i := 1       
				if i > 1 {   
					return i 
				}            
				             
				//notest
				             // *
				if i > 2 {   // *
					return i // *
				}            // *
				return 0     // *
			}
			`,
		"scope if": `package foo
			
			func Baz(i int) int { 
				if i > 2 {
					//notest
					return i // *
				}
				return 0
			}
			`,
		"scope file": `package foo
			
			//notest
			                      // *
			func Baz(i int) int { // *
				if i > 2 {        // *
					return i      // *
				}                 // *
				return 0          // *
			}                     // *
			                      // *
			func Foo(i int) int { // *
				return 0          // *
			}
			`,
		"complex comments": `package foo
			
			type Logger struct {
				Enabled bool
			}
			func (l Logger) Print(i ...interface{}) {}
			
			func Foo() {
				var logger Logger
				var tokens []interface{}
				if logger.Enabled {
					// notest
					for i, token := range tokens {        // *
						logger.Print("[", i, "] ", token) // *
					}                                     // *
				}
			}
			`,
		"case block": `package foo
			
			func Foo() bool {
				switch {
				case true:
					// notest
					if true {       // *
						return true // *
					}               // *
					return false    // *
				}
				return false
			}
			`,
	}
	test(t, tests)
}

func test(t *testing.T, tests map[string]string) {
	for name, source := range tests {
		env := vos.Mock()
		b, err := builder.New(env, "ns", true)
		if err != nil {
			t.Fatalf("Error creating builder in %s: %+v", name, err)
		}
		defer b.Cleanup()

		ppath, pdir, err := b.Package("a", map[string]string{
			"a.go": source,
		})
		if err != nil {
			t.Fatalf("Error creating package in %s: %+v", name, err)
		}

		paths := shared.NewCache(env)
		setup := &shared.Setup{
			Env:   env,
			Paths: paths,
		}
		if err := setup.Parse([]string{ppath}); err != nil {
			t.Fatalf("Error parsing args in %s: %+v", name, err)
		}

		cm := scanner.New(setup)

		if err := cm.LoadProgram(); err != nil {
			t.Fatalf("Error loading program in %s: %+v", name, err)
		}

		if err := cm.ScanPackages(); err != nil {
			t.Fatalf("Error scanning packages in %s: %+v", name, err)
		}

		result := cm.Excludes[filepath.Join(pdir, "a.go")]

		// matches strings like:
		//   - //notest$
		//   - // notest$
		//   - //notest // because this is glue code$
		//   - // notest // because this is glue code$
		notest := regexp.MustCompile("//\\s?notest(\\s//\\s?.*)?$")
		notestdept := regexp.MustCompile("//\\s?notestdept(\\s//\\s?.*)?$")
		

		for i, line := range strings.Split(source, "\n") {
			var expected shared.ExcludeType
			if strings.HasSuffix(line, "// *") || notest.MatchString(line) {
				expected = shared.Notest
			}
			if notestdept.MatchString(line) {
				expected = shared.Notestdept
			}
			if result[i+1] != expected {
				t.Fatalf("Unexpected state in %s, line %d: %s\n", name, i, strconv.Quote(strings.Trim(line, "\t")))
			}
		}

	}
}
