package rpm

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gezimbll/copr_builds/utils"
)

func GenerateFiles(errc chan<- error, filech chan<- string, owner, chroot, project string, version string, build int) {
	urlPath, err := url.JoinPath(utils.DownloadUrl, owner, project, chroot, fmt.Sprintf("0%v", build)+"-cgrates", utils.Prefix+strings.Join([]string{version, utils.ArchBuild, utils.RpmSuffix}, "."))
	if err != nil {
		errc <- err
		return
	}
	file, err := DownloadFile(strings.Join([]string{version, utils.ArchBuild, utils.RpmSuffix}, "."), project, chroot, urlPath)
	if err != nil {
		errc <- err
		return
	}
	filech <- file
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
	curr := filepath.Join(dirPath, strings.Join([]string{utils.Current, utils.RpmSuffix}, "."))
	if _, err = os.Stat(curr); err == nil {
		if err = os.Remove(curr); err != nil {
			return
		}
	}
	filePath = filepath.Join(dirPath, utils.Prefix+fileName)
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
