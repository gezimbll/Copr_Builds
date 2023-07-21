package rpm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DownloadUrl = "https://download.copr.fedorainfracloud.org/results/"
	CoprApi     = "http://copr.fedorainfracloud.org/api_3/package"
	Prefix      = "cgrates-"
	RpmSuffix   = "rpm"
	ArchBuild   = "x86_64"
	Current     = "cgrates-current"
	PackageDir  = "/var/packages/rpm"
)

type Builds struct {
	LatestSucceded struct {
		Id            int      `json:"id"`
		RepoUrl       string   `json:"repo_url"`
		Chroots       []string `json:"chroots"`
		SourcePackage struct {
			Name    string `json:"name"`
			Url     string `json:"url"`
			Version string `json:"version"`
		} `json:"source_package"`
	} `json:"latest_succeeded"`
}
type CoprPackage struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Ownername   string `json:"ownername"`
	ProjectName string `json:"projectname"`
	Builds      Builds `json:"builds"`
}

func GenerateFiles(chroot string, project string) (file string, err error) {
	var (
		c   CoprPackage
		req *http.Request
	)

	baseUrl, err := url.Parse(CoprApi)
	if err != nil {
		log.Printf("<%v> parsing url", err)
	}

	params := url.Values{
		"ownername":                   {"cgrates"},
		"projectname":                 {project},
		"packagename":                 {"cgrates"},
		"with_latest_succeeded_build": {"true"},
	}

	baseUrl.RawQuery = params.Encode()
	req, err = http.NewRequest(http.MethodGet, baseUrl.String(), nil)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&c)
	if err != nil {
		log.Println("Error decoding")
	}

	urlPath, err := url.JoinPath(DownloadUrl, c.Ownername, c.ProjectName, chroot, fmt.Sprintf("0%v", c.Builds.LatestSucceded.Id)+"-cgrates", Prefix+strings.Join([]string{c.Builds.LatestSucceded.SourcePackage.Version, ArchBuild, RpmSuffix}, "."))
	if err != nil {
		log.Fatal(err)
	}
	if file, err = DownloadFile(strings.Join([]string{c.Builds.LatestSucceded.SourcePackage.Version, ArchBuild, RpmSuffix}, "."), c.ProjectName, chroot, urlPath); err != nil {
		log.Printf("Failed to download file: %v", err)
		return
	}
	return
}

func DownloadFile(fileName, projectName, chroot, url string) (filePath string, err error) {
	var (
		resp *http.Response
		file *os.File
	)
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	fmt.Println(url)
	defer resp.Body.Close()
	tmp := os.TempDir()
	dirPath := filepath.Join(tmp, projectName, chroot)
	if err = os.MkdirAll(dirPath, 0775); err != nil {
		return
	}
	curr := filepath.Join(dirPath, strings.Join([]string{Current, RpmSuffix}, "."))
	if _, err = os.Stat(curr); err == nil {
		if err = os.Remove(curr); err != nil {
			return
		}
	}
	filePath = filepath.Join(dirPath, Prefix+fileName)
	if file, err = os.Create(filePath); err != nil {
		return
	}
	if _, err = io.Copy(file, resp.Body); err != nil {
		return
	}
	err = os.Symlink(filePath, curr)
	if err != nil {
		log.Fatalf("Failed to create symlink: %s", err)
	}
	return
}
