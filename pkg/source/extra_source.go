package source

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
)

func ProcessExtraSrcDirs(dirs []string, workDir string) ([]Artifact, error) {
	var artifacts []Artifact

	for _, dir := range dirs {
		log.Infof("creating tar from extra source directory %s", dir)
		tmpPath := filepath.Join(workDir, "extra-src.tar")

		checksum, err := createTar(tmpPath, dir)
		if err != nil {
			return nil, fmt.Errorf("creating tar for %s: %w", dir, err)
		}

		tarName := fmt.Sprintf("extra-src-%s.tar", checksum)
		tarPath := filepath.Join(workDir, tarName)
		if err := os.Rename(tmpPath, tarPath); err != nil {
			return nil, fmt.Errorf("renaming tar for %s: %w", dir, err)
		}
		log.Debugf("  tar checksum=%s", checksum)

		artifacts = append(artifacts, Artifact{
			Path:       tarPath,
			DriverName: "extra_src_dir",
			Annotations: map[string]string{
				"source.artifact.name":     tarName,
				"source.artifact.mimetype": "application/x-tar",
			},
		})
	}

	return artifacts, nil
}

func createTar(tarPath string, srcDir string) (string, error) {
	f, err := os.Create(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	w := io.MultiWriter(f, h)
	tw := tar.NewWriter(w)

	var paths []string
	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel != "." {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(paths)

	for _, rel := range paths {
		fullPath := filepath.Join(srcDir, rel)

		info, err := os.Lstat(fullPath)
		if err != nil {
			return "", err
		}

		var link string
		//TODO
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(fullPath)
			if err != nil {
				return "", err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return "", err
		}
		header.Name = rel
		header.ModTime = time.Unix(0, 0)
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if info.IsDir() {
			header.Mode = 0o777
		} else {
			header.Mode = 0o666
		}

		if err := tw.WriteHeader(header); err != nil {
			return "", err
		}

		if info.IsDir() {
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", err
		}
		if _, err := tw.Write(data); err != nil {
			return "", err
		}
	}

	if err := tw.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
