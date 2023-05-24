package helm

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func LoadChart(distro string) (*chart.Chart, error) {
	path := "charts/" + distro
	filenames, err := GetFilesRecursive(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("chart '%q' does not exist", distro)
		}
		return nil, fmt.Errorf("get files errors: %v", err)
	}

	var bfs []*loader.BufferedFile
	for _, filename := range filenames {
		b, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read file: %v", err)
		}
		// Helm expects unix / separator, but on windows this will be \
		name := strings.ReplaceAll(stripPrefix(filename, path), string(filepath.Separator), "/")
		bf := &loader.BufferedFile{
			Name: name,
			Data: b,
		}
		bfs = append(bfs, bf)
	}
	c, err := loader.LoadFiles(bfs)
	if err != nil {
		return nil, fmt.Errorf("load files: %v", err)
	}
	return c, nil
}

func GetFilesRecursive(root string) ([]string, error) {
	res := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		res = append(res, path)
		return nil
	})
	return res, err
}

// stripPrefix removes the given prefix from prefix.
func stripPrefix(path, prefix string) string {
	pl := len(strings.Split(prefix, "/"))
	pv := strings.Split(path, "/")
	return strings.Join(pv[pl:], "/")
}
