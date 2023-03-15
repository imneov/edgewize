package helm

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"kubesphere.io/kubesphere/manifests"
)

// TODO 重构
func LoadChart(distro string) (*chart.Chart, error) {
	path := "charts/" + distro
	filesystem := manifests.BuiltinOrDir("")
	filenames, err := GetFilesRecursive(filesystem, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("distro %q does not exist", distro)
		}
		return nil, fmt.Errorf("list files: %v", err)
	}

	var bfs []*loader.BufferedFile
	for _, filename := range filenames {
		b, err := fs.ReadFile(filesystem, filename)
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

func GetFilesRecursive(f fs.FS, root string) ([]string, error) {
	res := []string{}
	err := fs.WalkDir(f, root, func(path string, d fs.DirEntry, err error) error {
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
