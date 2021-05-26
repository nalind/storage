package storage

import (
	stderrors "errors"
	"io"
	"os"

	drivers "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	ErrLayerMissing                = stderrors.New("layer missing")
	ErrLayerUnreferenced           = stderrors.New("layer not used by images or containers") // detect
	ErrLayerIncorrectContentDigest = stderrors.New("layer content incorrect digest")
	ErrLayerIncorrectContentSize   = stderrors.New("layer content incorrect size")
	ErrLayerDataMissing            = stderrors.New("layer data item is missing")
	ErrImageLayerMissing           = stderrors.New("image layer is missing")
	ErrImageDataMissing            = stderrors.New("image data item is missing")
	ErrImageDataIncorrectSize      = stderrors.New("image data item has incorrect size")
	ErrContainerImageMissing       = stderrors.New("image missing")
	ErrContainerDataMissing        = stderrors.New("container data item is missing")
	ErrContainerDataIncorrectSize  = stderrors.New("container data item has incorrect size")
)

// CheckOptions is the set of options for Check.  It is currently a
// placeholder.
type CheckOptions struct {
	LayerDigests   bool
	LayerMountable bool
	LayerData      bool
	ImageData      bool
	ContainerData  bool
}

// CheckEverything returns a CheckOptions with everything enabled.
func CheckEverything() *CheckOptions {
	return &CheckOptions{
		LayerDigests:   true,
		LayerMountable: true,
		LayerData:      true,
		ImageData:      true,
		ContainerData:  true,
	}
}

// CheckReport is a list of problems.
type CheckReport struct {
	Layers     map[string][]error
	Images     map[string][]error
	Containers map[string][]error
}

// RepairOptions is the set of options for Repair.
type RepairOptions struct {
	Containers bool // Remove containers which use damaged layers
}

// RepairEverything returns a RepairOptions with everything enabled.
func RepairEverything() *RepairOptions {
	return &RepairOptions{
		Containers: true,
	}
}

// Check returns a list of problems with what's in the store, as a whole.  It
// will be very expensive to call.
func (s *store) Check(options *CheckOptions) (CheckReport, error) {
	if options == nil {
		options = CheckEverything()
	}

	storesLock.Lock()
	defer storesLock.Unlock()

	rcstore, err := s.ContainerStore()
	if err != nil {
		return CheckReport{}, err
	}

	istore, err := s.ImageStore()
	if err != nil {
		return CheckReport{}, err
	}

	lstore, err := s.LayerStore()
	if err != nil {
		return CheckReport{}, err
	}

	lstores, err := s.ROLayerStores()
	if err != nil {
		return CheckReport{}, err
	}
	for _, s := range append([]ROLayerStore{lstore}, lstores...) {
		store := s
		store.RLock()
		defer store.Unlock()
		if modified, err := store.Modified(); modified || err != nil {
			if err = store.Load(); err != nil {
				return CheckReport{}, err
			}
		}
	}

	istores, err := s.ROImageStores()
	if err != nil {
		return CheckReport{}, err
	}
	for _, s := range append([]ROImageStore{istore}, istores...) {
		store := s
		store.RLock()
		defer store.Unlock()
		if modified, err := store.Modified(); modified || err != nil {
			if err = store.Load(); err != nil {
				return CheckReport{}, err
			}
		}
	}

	rcstore.Lock()
	defer rcstore.Unlock()
	if modified, err := rcstore.Modified(); modified || err != nil {
		if err = rcstore.Load(); err != nil {
			return CheckReport{}, err
		}
	}

	layerParents := make(map[string]string)
	report := CheckReport{
		Layers:     make(map[string][]error),
		Images:     make(map[string][]error),
		Containers: make(map[string][]error),
	}

	// Walk the list of layer stores, looking at each layer that we didn't
	// see in a previously-visited store.
	examinedLayers := make(map[string]struct{})
	for _, store := range append([]ROLayerStore{lstore}, lstores...) {
		layers, err := store.Layers()
		if err != nil {
			return CheckReport{}, err
		}
		// Iterate over each layer in turn.
		for j := range layers {
			layer := layers[j]
			id := layer.ID
			// If we've already seen a layer with this ID, skip it.
			if _, checked := examinedLayers[id]; checked {
				continue
			}
			// Note the parent of this layer, and that we've
			// visited it.
			layerParents[id] = layer.Parent
			examinedLayers[id] = struct{}{}
			logrus.Debugf("layer %s", id)
			// Check that all of the big data items are present and
			// reading them back gives us the right amount of data.
			if options.LayerData {
				func() {
					for _, name := range layer.BigDataNames {
						rc, err := store.BigData(id, name)
						if err != nil {
							if os.IsNotExist(errors.Cause(err)) {
								report.Layers[id] = append(report.Layers[id], errors.Wrapf(ErrLayerDataMissing, "layer %s", id))
								return
							}
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
							return
						}
						defer rc.Close()
						if _, err = io.Copy(io.Discard, rc); err != nil {
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
							return
						}
					}
				}()
			}
			// At this point we're out of things that we can be
			// sure will work in read-only stores, so skip any
			// stores that aren't also read-write stores.
			rwstore, ok := store.(LayerStore)
			if !ok {
				continue
			}
			// Check that the content we get back when extracting
			// the layer's contents match the recorded digest and
			// size.  A layer for which they're not given isn't a
			// part of an image, and is likely the read-write layer
			// for a container, and we're not responsible for
			// their contents.
			if options.LayerDigests {
				func() {
					if layer.UncompressedDigest != "" {
						expectedDigest := layer.UncompressedDigest
						if err := layer.UncompressedDigest.Validate(); err != nil {
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
							return
						}
						digester := expectedDigest.Algorithm().Digester()
						uncompressed := archive.Uncompressed
						diffOptions := DiffOptions{
							Compression: &uncompressed,
						}
						diff, err := rwstore.Diff("", id, &diffOptions)
						if err != nil {
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
							return
						}
						defer diff.Close()
						reader := io.TeeReader(diff, digester.Hash())
						n, err := io.Copy(io.Discard, reader)
						if err != nil {
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
							return
						}
						if digester.Digest() != layer.UncompressedDigest {
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(ErrLayerIncorrectContentDigest, "layer %s", id))
							return
						}
						if layer.UncompressedSize != -1 && n != layer.UncompressedSize {
							report.Layers[id] = append(report.Layers[id], errors.Wrapf(ErrLayerIncorrectContentSize, "layer %s", id))
							return
						}
					}
				}()
			}
			// FIXME: break the section above into its own loop to
			// summarize all of the diffs completely, then make the
			// next bit use them to build an mtree or other
			// structure that catalogs the expected contents of the
			// whole tree.  Compare to what we see when we mount
			// the layer and walk the tree, and flag cases where
			// content is in the layer that shouldn't be there.
			// The implementation of Diff() won't catch this
			// problem by itself.
			if options.LayerMountable {
				func() {
					if _, err := rwstore.Mount(id, drivers.MountOpts{MountLabel: layer.MountLabel}); err != nil {
						report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
						return
					}
					if _, err := rwstore.Unmount(id, false); err != nil {
						report.Layers[id] = append(report.Layers[id], errors.Wrapf(err, "layer %s", id))
						return
					}
				}()
			}
		}
	}

	// Walk the list of image stores, looking at each image that we didn't
	// see in a previously-visited store.
	examinedImages := make(map[string]struct{})
	for _, store := range append([]ROImageStore{istore}, istores...) {
		images, err := store.Images()
		if err != nil {
			return CheckReport{}, err
		}
		// Iterate over each image in turn.
		for j := range images {
			image := images[j]
			id := image.ID
			// If we've already seen an image with this ID, skip it.
			if _, checked := examinedImages[id]; checked {
				continue
			}
			examinedImages[id] = struct{}{}
			logrus.Debugf("image %s", id)
			if options.ImageData {
				// Check that all of the big data items are present and
				// reading them back gives us the right amount of data.
				// Even though we record digests, we don't know how
				// they were calculated, so do not try to check them.
				func() {
					for _, key := range image.BigDataNames {
						data, err := store.BigData(id, key)
						if err != nil {
							if os.IsNotExist(errors.Cause(err)) {
								report.Images[id] = append(report.Images[id], errors.Wrapf(ErrImageDataMissing, "image %s", id))
								return
							}
							report.Images[id] = append(report.Images[id], errors.Wrapf(err, "image %s", id))
							return
						}
						if int64(len(data)) != image.BigDataSizes[key] {
							report.Images[id] = append(report.Images[id], errors.Wrapf(ErrImageDataIncorrectSize, "image %s", id))
							return
						}
					}
				}()
			}
			// Walk the layers list for the image.  For every layer
			// that the image uses that has errors, the layer's
			// errors are the image's errors.
			examinedImageLayers := make(map[string]struct{})
			for _, topLayer := range append([]string{image.TopLayer}, image.MappedTopLayers...) {
				if topLayer == "" {
					continue
				}
				if _, checked := examinedImageLayers[topLayer]; checked {
					continue
				}
				examinedImageLayers[topLayer] = struct{}{}
				for layer := topLayer; layer != ""; layer = layerParents[layer] {
					if _, checked := examinedLayers[layer]; !checked {
						report.Images[id] = append(report.Images[id], errors.WithMessagef(errors.Wrapf(ErrImageLayerMissing, "layer %s", layer), "image %s", id))
					}
					report.Images[id] = append(report.Images[id], report.Layers[layer]...)
				}
			}
		}
	}

	// Iterate over each container in turn.
	containers, err := rcstore.Containers()
	if err != nil {
		return CheckReport{}, err
	}
	for i := range containers {
		container := containers[i]
		id := container.ID
		logrus.Debugf("container %s", id)
		if options.ContainerData {
			func() {
				// Check that all of the big data items are present and
				// reading them back gives us the right amount of data.
				for _, key := range container.BigDataNames {
					data, err := rcstore.BigData(id, key)
					if err != nil {
						if os.IsNotExist(errors.Cause(err)) {
							report.Containers[id] = append(report.Containers[id], errors.Wrapf(ErrContainerDataMissing, "container %s", id))
							return
						}
						report.Containers[id] = append(report.Containers[id], errors.Wrapf(err, "container %s", id))
						return
					}
					if int64(len(data)) != container.BigDataSizes[key] {
						report.Containers[id] = append(report.Containers[id], errors.Wrapf(ErrContainerDataIncorrectSize, "container %s", id))
						return
					}
				}
			}()
		}
		// Look at the container's base image.  If the image has
		// errors, the image's errors are the container's errors.
		if container.ImageID != "" {
			if _, checked := examinedImages[container.ImageID]; !checked {
				report.Containers[id] = append(report.Containers[id], errors.Wrapf(ErrContainerImageMissing, "image %s", container.ImageID))
			}
			report.Containers[id] = append(report.Containers[id], report.Images[container.ImageID]...)
		}
	}
	return report, nil
}

// Repair removes items which are not themselves incorrect, or which depend on
// items which are not correct.
func (s *store) Repair(report CheckReport, options *RepairOptions) []error {
	if options == nil {
		options = RepairEverything()
	}
	var errs []error
	if options.Containers {
		for id := range report.Containers {
			errs = append(errs, errors.Wrapf(s.DeleteContainer(id), "deleting container"))
		}
	}
	deletedLayers := make(map[string]struct{})
	for id := range report.Images {
		layers, err := s.DeleteImage(id, true)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "deleting image"))
		} else {
			for _, layer := range layers {
				logrus.Infof("deleted layer %q", layer)
				deletedLayers[layer] = struct{}{}
			}
		}
	}
	for id := range report.Layers {
		if _, ok := deletedLayers[id]; ok {
			continue
		}
		errs = append(errs, errors.Wrapf(s.DeleteLayer(id), "deleting layer"))
	}
	return errs
}
