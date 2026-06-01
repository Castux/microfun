package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// differential_test.go is the equivalence gate between the two backends. Both
// must produce byte-identical stdout/stderr and the same exit code on every
// program, so each test runs a file under --mode=interp and --mode=compiled and
// diffs the combined output. See BYTECODE.md §13.

var testBinary string

func TestMain(m *testing.M) {
	bin := "microfun_test_bin"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	dir, err := os.MkdirTemp("", "microfun-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	testBinary = filepath.Join(dir, bin)

	build := exec.Command("go", "build", "-o", testBinary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("building test binary failed: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

// run executes the test binary on path in the given mode with the given stdin,
// returning combined output and exit code.
func run(t *testing.T, mode, path, stdin string) (string, int) {
	t.Helper()
	cmd := exec.Command(testBinary, "--mode="+mode, path)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running %s in %s mode: %v", path, mode, err)
		}
	}
	return string(out), code
}

// assertParity runs path in both modes with the given stdin and fails if the
// output or exit code differ.
func assertParity(t *testing.T, path, stdin string) {
	t.Helper()
	interpOut, interpCode := run(t, "interp", path, stdin)
	compiledOut, compiledCode := run(t, "compiled", path, stdin)
	if interpCode != compiledCode {
		t.Errorf("%s: exit code differs: interp=%d compiled=%d", path, interpCode, compiledCode)
	}
	if interpOut != compiledOut {
		t.Errorf("%s: output differs:\n--- interp ---\n%s\n--- compiled ---\n%s", path, interpOut, compiledOut)
	}
}

// collectPrograms gathers every .mf file under the given roots.
func collectPrograms(t *testing.T, roots ...string) []string {
	t.Helper()
	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(path) == ".mf" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	return files
}

// TestDifferential runs the example programs and the feature/error corpus in
// both backends with a fixed UTF-8 stdin (ignored by programs that do not read
// it) and asserts byte-identical results.
func TestDifferential(t *testing.T) {
	const stdin = "Héllo, wörld!\nsecond line\n"
	for _, path := range collectPrograms(t, "examples", "testdata") {
		path := path
		t.Run(path, func(t *testing.T) {
			assertParity(t, path, stdin)
		})
	}
}

// TestStdinVariants exercises the stdin-reading programs with several inputs,
// including the invalid-UTF-8 case that is a runtime error in both modes.
func TestStdinVariants(t *testing.T) {
	cases := []string{
		"",
		"a",
		"Hello\nWorld\n",
		"Héllo wörld 日本語\n",
		"\xff\xfe",       // invalid UTF-8: runtime error for stdin code points
		"\x00\x01\x02\n", // control bytes
	}
	for _, path := range collectPrograms(t, "testdata/stdin") {
		for i, input := range cases {
			path, input := path, input
			t.Run(filepath.Base(path)+"#"+strconv.Itoa(i), func(t *testing.T) {
				assertParity(t, path, input)
			})
		}
	}
}

// TestCoreTestsBothModes runs the in-language self-checking test harness in both
// modes; both must print the same PASS/FAIL report.
func TestCoreTestsBothModes(t *testing.T) {
	assertParity(t, filepath.Join("examples", "core_tests.mf"), "")
}
