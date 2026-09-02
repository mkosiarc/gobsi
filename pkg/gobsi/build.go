package gobsi

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mkosiarc/gobsi/pkg/oci"
	"github.com/mkosiarc/gobsi/pkg/source"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
)

type BuildConfig struct {
	SRPMDir   string
	ExtraDirs []string
	OutputDir string
}

func BuildSourceImage(cfg BuildConfig) error {
	if cfg.SRPMDir == "" && len(cfg.ExtraDirs) == 0 {
		return fmt.Errorf("provide at least one input: SRPMDir or ExtraDirs")
	}

	log.Infof("building source image to %s", cfg.OutputDir)

	if err := oci.CreateOCIDirectory(cfg.OutputDir); err != nil {
		return fmt.Errorf("creating OCI directory: %w", err)
	}

	var artifacts []source.Artifact

	if cfg.SRPMDir != "" {
		log.Infof("processing SRPMs from %s", cfg.SRPMDir)
		srpms, err := source.ProcessSRPMDir(cfg.SRPMDir)
		if err != nil {
			return fmt.Errorf("processing SRPMs: %w", err)
		}
		log.Debugf("found %d SRPMs", len(srpms))
		artifacts = append(artifacts, srpms...)
	}

	if len(cfg.ExtraDirs) > 0 {
		log.Infof("processing %d extra source directories", len(cfg.ExtraDirs))
		workDir, err := os.MkdirTemp("", "gobsi-")
		if err != nil {
			return fmt.Errorf("creating work directory: %w", err)
		}
		defer os.RemoveAll(workDir)

		extras, err := source.ProcessExtraSrcDirs(cfg.ExtraDirs, workDir)
		if err != nil {
			return fmt.Errorf("processing extra sources: %w", err)
		}
		log.Debugf("created %d extra source tars", len(extras))
		artifacts = append(artifacts, extras...)
	}

	config := oci.NewConfig()
	manifest := oci.NewManifest()

	for _, a := range artifacts {
		log.Infof("adding layer for %s", a.Annotations["source.artifact.name"])
		layer, err := oci.CreateLayer(cfg.OutputDir, a.Path, a.DriverName, a.Annotations)
		if err != nil {
			return fmt.Errorf("creating layer for %s: %w", a.Path, err)
		}
		manifest.Layers = append(manifest.Layers, layer.Descriptor)
		config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, layer.DiffID)
		log.Debugf("layer digest=%s size=%d", layer.Descriptor.Digest, layer.Descriptor.Size)

		now := time.Now()
		checksum := strings.TrimPrefix(layer.Descriptor.Annotations["source.artifact.filename.checksum"], "sha256:")
		config.History = append(config.History, ocispec.History{
			Created:   &now,
			CreatedBy: fmt.Sprintf("#(nop) gobsi adding artifact: %s", checksum),
		})
	}

	now := time.Now()
	config.Created = &now

	configDigest, configSize, err := oci.SaveConfig(cfg.OutputDir, config)
	if err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	log.Debugf("config digest=%s size=%d", configDigest, configSize)

	manifest.Config = ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    digest.Digest(configDigest),
		Size:      configSize,
	}

	manifestDigest, manifestSize, err := oci.SaveManifest(cfg.OutputDir, manifest)
	if err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}
	log.Debugf("manifest digest=%s size=%d", manifestDigest, manifestSize)

	if err := oci.SaveIndex(cfg.OutputDir, manifestDigest, manifestSize); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	log.Infof("source image successfully built at %s", cfg.OutputDir)
	return nil
}
