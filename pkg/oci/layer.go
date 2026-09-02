package oci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TODO doc
type LayerInfo struct {
	Descriptor ocispec.Descriptor
	DiffID     digest.Digest
}

// TODO doc
func CreateLayer(ociDir string, artifactPath string, driverName string, annotations map[string]string) (LayerInfo, error) {

	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		return LayerInfo{}, err
	}

	artifactHash := fmt.Sprintf("%x", sha256.Sum256(artifactData))
	artifactName := filepath.Base(artifactPath)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	dirs := []string{
		"./",
		"./blobs/",
		"./blobs/sha256/",
		"./" + driverName + "/",
	}
	for _, d := range dirs {
		if err := tw.WriteHeader(&tar.Header{
			Name:     d,
			Typeflag: tar.TypeDir,
			Mode:     0o777,
			Uname:    "root",
			Gname:    "root",
		}); err != nil {
			return LayerInfo{}, err
		}
	}

	blobPath := fmt.Sprintf("./blobs/sha256/%s", artifactHash)
	if err := tw.WriteHeader(&tar.Header{
		Name:     blobPath,
		Size:     int64(len(artifactData)),
		Mode:     0o666,
		Typeflag: tar.TypeReg,
		Uname:    "root",
		Gname:    "root",
	}); err != nil {
		return LayerInfo{}, err
	}
	if _, err := tw.Write(artifactData); err != nil {
		return LayerInfo{}, err
	}

	linkTarget := fmt.Sprintf("../blobs/sha256/%s", artifactHash)
	if err := tw.WriteHeader(&tar.Header{
		Name:     fmt.Sprintf("./%s/%s", driverName, artifactName),
		Typeflag: tar.TypeSymlink,
		Linkname: linkTarget,
		Mode:     0o777,
		Uname:    "root",
		Gname:    "root",
	}); err != nil {
		return LayerInfo{}, err
	}

	if err := tw.Close(); err != nil {
		return LayerInfo{}, err
	}

	tarData := buf.Bytes()
	tarDigest, tarSize, err := SaveBlob(ociDir, tarData)
	if err != nil {
		return LayerInfo{}, err
	}

	diffID := digest.Digest(fmt.Sprintf("sha256:%x", sha256.Sum256(tarData)))

	annotations["source.artifact.filename.checksum"] = fmt.Sprintf("sha256:%s", artifactHash)

	return LayerInfo{
		Descriptor: ocispec.Descriptor{
			MediaType:   ocispec.MediaTypeImageLayer,
			Digest:      digest.Digest(tarDigest),
			Size:        tarSize,
			Annotations: annotations,
		},
		DiffID: diffID,
	}, nil
}
