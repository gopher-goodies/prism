package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/geckoboard/prism/inject"
)

//
func ProfileProject(ctx *cli.Context) {
	args := ctx.Args()
	if len(args) != 1 {
		exitWithError("error: missing path_to_main_file argument")
	}

	profileFuncs := ctx.StringSlice("target")
	if len(profileFuncs) == 0 {
		exitWithError("error: no profile targets specified")
	}

	origPathToMain := args[0]
	origProjectPath, err := filepath.Abs(filepath.Dir(origPathToMain))
	if err != nil {
		exitWithError(err.Error())
	}
	origProjectPath += "/"

	// Clone project
	tmpDir, tmpPathToMain, err := cloneProject(origPathToMain, ctx.String("output-folder"))
	if err != nil {
		exitWithError(err.Error())
	}
	if !ctx.Bool("preserve-output") {
		defer deleteClonedProject(tmpDir)
	}

	// Analyze project
	analyzer, err := inject.NewAnalyzer(tmpPathToMain, origProjectPath)
	if err != nil {
		exitWithError(err.Error())
	}

	// Select profile targets
	profileTargets, err := analyzer.ProfileTargets(profileFuncs)
	fmt.Printf("profile: call graph analyzed %d target(s) and detected %d locations for injecting profiler hooks\n", len(profileFuncs), len(profileTargets))

	// Inject profiler
	injector := inject.NewInjector(filepath.Dir(tmpPathToMain), origProjectPath)
	touchedFiles, err := injector.Hook(profileTargets)
	if err != nil {
		exitWithError(err.Error())
	}

	fmt.Printf("profile: updated %d files\n", touchedFiles)
}

// Clone project and return updated path to main
func cloneProject(pathToMain, dest string) (tmpDir, tmpPathToMain string, err error) {
	mainFile := filepath.Base(pathToMain)

	// Get absolute project path and trim everything before the first "src/"
	// path segment which indicates the root of the GOPATH where the project resides in.
	absProjectPath, err := filepath.Abs(filepath.Dir(pathToMain))
	if err != nil {
		return "", "", err
	}
	skipLen := strings.Index(absProjectPath, "/src/")

	tmpDir, err = ioutil.TempDir(dest, "prism-")
	if err != nil {
		return "", "", err
	}

	fmt.Printf("profile: copying project to %s\n", tmpDir)

	err = filepath.Walk(absProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		dstPath := tmpDir + path[skipLen:]

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		} else if !info.Mode().IsRegular() {
			fmt.Printf("profile: [WARNING] skipping non-regular file %s\n", path)
			return nil
		}

		// Copy file
		fSrc, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fSrc.Close()
		fDst, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer fDst.Close()
		_, err = io.Copy(fDst, fSrc)
		return err
	})

	if err != nil {
		deleteClonedProject(tmpDir)
		return "", "", err
	}

	return tmpDir,
		tmpDir + absProjectPath[skipLen:] + "/" + mainFile,
		nil
}

func deleteClonedProject(path string) {
	os.RemoveAll(path)
}