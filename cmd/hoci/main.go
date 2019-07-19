package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"hoci"
	"log"
	"os"
	"os/exec"
)

type Package struct {
	Name    string        `json:"name" dpkg:"binary:Package"`
	Version string        `json:"version" dpkg:"Version"`
	Arch    string        `json:"arch" dpkg:"Architecture"`
	Source  SourcePackage `json:"source"`
	Summary string        `json:"summary" dpkg:"binary:Summary"`
}

type SourcePackage struct {
	Name            string `json:"name" dpkg:"source:Package"`
	Version         string `json:"version" dpkg:"source:Version"`
	UpstreamVersion string `json:"upstreamVersion" dpkg:"source:Upstream-Version"`
}

func main() {
	var image string
	flag.StringVar(&image, "p", "", "image name to label with packages")
	flag.Parse()
	logger := log.New(os.Stderr, "", log.LstdFlags)
	dpkg := hoci.DPKG{
		Log: logger,
		Query: func(query string) *exec.Cmd {
			return exec.Command("docker", "run", "--rm", image, "dpkg-query", "-W", "-f="+query)
		},
	}
	var pkgs []Package
	err := dpkg.Metadata(&pkgs)
	if err != nil {
		logger.Fatal(err)
	}

	out, err := json.Marshal(pkgs)
	if err != nil {
		logger.Fatal(err)
	}
	cmd := exec.Command("docker", "build", "-t", image, "--label", "sh.scl.hoci.packages="+string(out), "-")
	cmd.Stdin = bytes.NewBufferString("FROM " + image + "\n")
	if err := cmd.Run(); err != nil {
		logger.Fatal(err)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		logger.Print(string(out))
		logger.Fatal(err)
	}
}
