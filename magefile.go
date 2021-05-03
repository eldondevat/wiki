// +build mage

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mholt/archiver"
)

const (
	ProjectName = "wiki"
	DistFolder  = "dist"
	TimeFormat  = "2006-01-02 15:04:05 -0700"
	GoExe       = "go"
	DockerEXE   = "docker"
	GitExe      = "git"
)

var (
	arches = []Architecture{
		{OS: "linux", Arch: "amd64", ArchiveType: ".tar.gz"},
		{OS: "linux", Arch: "arm64", ArchiveType: ".tar.gz"},
		{OS: "darwin", Arch: "amd64", ArchiveType: ".tar.gz"},
		{OS: "darwin", Arch: "arm64", ArchiveType: ".tar.gz"},
		{OS: "windows", Arch: "amd64", BinarySuffix: ".exe", ArchiveType: ".zip"},
	}
	options = CompilerOptions{
		GCFlags: []string{
			`./dontoptimizeme=-N`,
		},
		LDFlags: []string{
			`-s`,
			`-w`,
			fmt.Sprintf(`-X "main.version=%s"`, buildTag),
		},
		MiscFlags: []string{
			`-trimpath`,
		},
	}
	isTag      = false
	semverTags []string
	buildTag   = "unknown"
	buildTime  = time.Time{}

	Default = Release.All
)

type Build mg.Namespace
type Release mg.Namespace

func init() {
	err := setBuildTime()
	if err != nil {
		os.Exit(1)
	}
	err = setBuildVersion()
	if err != nil {
		os.Exit(1)
	}
	err = setSemVerTags()
	if err != nil {
		os.Exit(1)
	}
}

func (Release) All() error {
	mg.Deps(Release.Docker, Release.Archive)
	return nil
}

func (Release) Docker() error {
	mg.Deps(Release.Notices, Build.LinuxAmd64)
	fmt.Printf("Building docker container\n")
	bytesRead, err := ioutil.ReadFile("gorelease.Dockerfile")
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(DistFolder, "Dockerfile"), bytesRead, 0755)
	if err != nil {
		log.Fatal(err)
	}
	err = sh.Run(DockerEXE, "build", "-t", ProjectName, DistFolder)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(DistFolder, "Dockerfile"))
	if err != nil {
		return err
	}
	for _, semverTag := range semverTags {
		err = sh.Run(DockerEXE, "tag", ProjectName, fmt.Sprintf("%s:%s", ProjectName, semverTag))
		if err != nil {
			return err
		}
	}
	return nil
}

func (Release) Archive() error {
	mg.Deps(Release.Notices, Build.All)
	fmt.Printf("Creating archives\n")
	for _, architecture := range arches {
		binaryName := fmt.Sprintf("%s_%s_%s_%s", ProjectName, architecture.OS, architecture.Arch, buildTag)
		outputName := filepath.Join(DistFolder, "archives", binaryName)
		err := archiver.Archive([]string{
			"dist/" + binaryName + architecture.BinarySuffix,
			"dist/notices",
		}, outputName+architecture.ArchiveType)
		if err != nil {
			log.Printf("Error archiving: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
		var data []byte
		data, err = os.ReadFile(outputName + architecture.ArchiveType)
		if err != nil {
			log.Printf("Error reading archive: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
		checksum := sha256.Sum256(data)
		err = os.WriteFile(outputName+"_checksum.sha256", []byte(fmt.Sprintf("%x", checksum)), 0644)
		if err != nil {
			log.Printf("Error writing checksum: %s%s: %s", architecture.OS, architecture.Arch, err.Error())
		}
	}
	return nil
}

func (Release) Notices() error {
	fmt.Printf("Getting licenses\n")
	noticesPath := filepath.Join(DistFolder, "notices")
	err := sh.Run(GoExe, "get", "")
	if err != nil {
		return err
	}
	err = sh.Run(GoExe, "get", "github.com/google/go-licenses")
	if err != nil {
		return err
	}
	err = sh.Run("go-licenses", "save", "./...", fmt.Sprintf("--save_path=%s", noticesPath), "--force")
	if err != nil {
		return err
	}
	return filepath.WalkDir(noticesPath, setTimeFunc(buildTime))
}

func (Build) All() error {
	mg.Deps(Build.LinuxAmd64, Build.LinuxArm64, Build.DarwinAmd64, Build.DarwinArm64, Build.WindowsAmd64)
	return nil
}

func (Build) WindowsAmd64() error {
	fmt.Printf("Building Windows AMD64\n")
	return build(Architecture{
		OS:           "windows",
		Arch:         "amd64",
		BinarySuffix: ".exe",
		ArchiveType:  ".zip",
	})
}

func (Build) LinuxAmd64() error {
	fmt.Printf("Building Linux AMD64\n")
	return build(Architecture{
		OS:           "linux",
		Arch:         "amd64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func (Build) LinuxArm64() error {
	fmt.Printf("Building Linux ARM64\n")
	return build(Architecture{
		OS:           "linux",
		Arch:         "arm64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func (Build) DarwinAmd64() error {
	fmt.Printf("Building Darwin AMD64\n")
	return build(Architecture{
		OS:           "darwin",
		Arch:         "amd64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func (Build) DarwinArm64() error {
	fmt.Printf("Building Darwin ARM64\n")
	return build(Architecture{
		OS:           "darwin",
		Arch:         "arm64",
		BinarySuffix: "",
		ArchiveType:  ".tar.gz",
	})
}

func setBuildVersion() error {
	var err error
	buildTag, err = getTag()
	if err != nil {
		return err
	}
	var exactTag string
	exactTag, isTag, err = getExactTag()
	if isTag {
		fmt.Printf("Tagged build: %s\n", exactTag)
	} else {
		fmt.Printf("Snapshot build: %s\n", buildTag)
	}
	return nil
}

func setBuildTime() error {
	var err error
	commitTimestamp, err := sh.Output(GitExe, "show", "-s", "--format=%ci", "HEAD")
	if err != nil {
		return err
	}
	buildTime, err = time.Parse(TimeFormat, commitTimestamp)
	if err != nil {
		return err
	}
	return nil
}

func setTimeFunc(buildtime time.Time) func(path string, info fs.DirEntry, err error) error {
	return func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		err = os.Chtimes(path, buildtime, buildtime)
		if err != nil {
			return err
		}
		return nil
	}
}

func setSemVerTags() error {
	if !isTag {
		semverTags = append(semverTags, "latest")
		return nil
	}
	buildTag = strings.TrimPrefix(buildTag, "v")
	semVer, err := semver.NewVersion(buildTag)
	if err != nil {
		fmt.Printf("Not a semver release: %s\n", err)
		return err
	}
	semverTags = append(semverTags, fmt.Sprintf("%d.%d.%d", semVer.Major, semVer.Minor, semVer.Patch))
	semverTags = append(semverTags, fmt.Sprintf("%d.%d", semVer.Major, semVer.Minor))
	semverTags = append(semverTags, fmt.Sprintf("%d", semVer.Major))
	return nil
}

func getTag() (string, error) {
	_, err := sh.Output(GitExe, "fetch", "--tags")
	if err != nil {
		return "", err
	}
	s, err := sh.Output(GitExe, "describe", "--tags")
	if err != nil {
		return "", err
	}
	return s, nil
}

func getExactTag() (string, bool, error) {
	_, err := sh.Output(GitExe, "fetch", "--tags")
	if err != nil {
		return "", false, err
	}
	buf := &bytes.Buffer{}
	ran, err := sh.Exec(nil, buf, nil, GitExe, "describe", "--exact-match", "--tags")
	if !ran && err != nil {
		return "", false, err
	}
	if ran && err != nil {
		return "", false, err
	}
	return buf.String(), true, err
}

func build(arch Architecture) error {
	err := os.Setenv("GOOS", arch.OS)
	if err != nil {
		return err
	}
	err = os.Setenv("GOARCH", arch.Arch)
	if err != nil {
		return err
	}
	outputName := arch.getOutputName()
	err = sh.RunV(GoExe, options.getAllFlags(outputName)...)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(outputName, setTimeFunc(buildTime))
	if err != nil {
		return err
	}
	return nil
}

type Architecture struct {
	OS           string
	Arch         string
	BinarySuffix string
	ArchiveType  string
}

func (a *Architecture) getOutputName() string {
	return filepath.Join(DistFolder, "binaries", fmt.Sprintf("%s_%s_%s_%s%s", ProjectName, a.OS, a.Arch, buildTag, a.BinarySuffix))
}

type CompilerOptions struct {
	GCFlags   []string
	LDFlags   []string
	MiscFlags []string
}

func (c *CompilerOptions) getAllFlags(output string) []string {
	buildFlags := []string{
		"build",
	}
	buildFlags = append(buildFlags, c.MiscFlags...)
	buildFlags = append(buildFlags, "-gcflags="+strings.Join(c.GCFlags, " "))
	buildFlags = append(buildFlags, "-ldflags="+strings.Join(c.LDFlags, " "))
	buildFlags = append(buildFlags, "-o")
	buildFlags = append(buildFlags, output)
	buildFlags = append(buildFlags, ".")
	return buildFlags
}
