package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	rpmlib "github.com/cavaliergopher/rpm"
	log "github.com/sirupsen/logrus"
)

type Artifact struct {
	Path        string
	DriverName  string
	Annotations map[string]string
}

func ProcessSRPMDir(srpmDir string) ([]Artifact, error) {
	var matches []string
	err := filepath.WalkDir(srpmDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, "src.rpm") {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var artifacts []Artifact
	for _, path := range matches {
		log.Infof("processing SRPM %s", filepath.Base(path))
		annotations, err := extractSRPMMetadata(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		log.Debugf("  name=%s version=%s epoch=%s", annotations["source.artifact.name"], annotations["source.artifact.version"], annotations["source.artifact.epoch"])
		artifacts = append(artifacts, Artifact{
			Path:        path,
			DriverName:  "rpm_dir",
			Annotations: annotations,
		})
	}

	return artifacts, nil
}

func extractSRPMMetadata(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pkg, err := rpmlib.Read(f)
	if err != nil {
		return nil, err
	}

	// TODO, what is happening here
	pkgID := fmt.Sprintf("%x", pkg.Signature.GetTag(1004).Bytes())

	annotations := map[string]string{
		"source.artifact.filename":  filepath.Base(path),
		"source.artifact.name":      pkg.Name(),
		"source.artifact.version":   pkg.Version(),
		"source.artifact.epoch":     strconv.Itoa(pkg.Epoch()),
		"source.artifact.release":   pkg.Release(),
		"source.artifact.license":   pkg.License(),
		"source.artifact.mimetype":  "application/x-rpm",
		"source.artifact.buildtime": strconv.FormatInt(pkg.BuildTime().Unix(), 10),
		"source.artifact.pkgid":     pkgID,
	}

	return annotations, nil

}
