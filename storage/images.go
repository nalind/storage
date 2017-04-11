package storage

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/stringid"
)

var (
	// ErrImageUnknown indicates that there was no image with the specified name or ID
	ErrImageUnknown = errors.New("image not known")
)

// An Image is a reference to a layer and an associated metadata string.
type Image struct {
	// ID is either one which was specified at create-time, or a random
	// value which was generated by the library.
	ID string `json:"id"`

	// Names is an optional set of user-defined convenience values.  The
	// image can be referred to by its ID or any of its names.  Names are
	// unique among images.
	Names []string `json:"names,omitempty"`

	// TopLayer is the ID of the topmost layer of the image itself.
	// Multiple images can refer to the same top layer.
	TopLayer string `json:"layer"`

	// Metadata is data we keep for the convenience of the caller.  It is not
	// expected to be large, since it is kept in memory.
	Metadata string `json:"metadata,omitempty"`

	// BigDataNames is a list of names of data items that we keep for the
	// convenience of the caller.  They can be large, and are only in
	// memory when being read from or written to disk.
	BigDataNames []string `json:"big-data-names,omitempty"`

	// BigDataSizes maps the names in BigDataNames to the sizes of the data
	// that has been stored, if they're known.
	BigDataSizes map[string]int64 `json:"big-data-sizes,omitempty"`

	Flags map[string]interface{} `json:"flags,omitempty"`
}

// ImageStore provides bookkeeping for information about Images.
type ImageStore interface {
	FileBasedStore
	MetadataStore
	BigDataStore
	FlaggableStore

	// Create creates an image that has a specified ID (or a random one) and
	// optional names, using the specified layer as its topmost (hopefully
	// read-only) layer.  That layer can be referenced by multiple images.
	Create(id string, names []string, layer, metadata string) (*Image, error)

	// SetNames replaces the list of names associated with an image with the
	// supplied values.
	SetNames(id string, names []string) error

	// Exists checks if there is an image with the given ID or name.
	Exists(id string) bool

	// Get retrieves information about an image given an ID or name.
	Get(id string) (*Image, error)

	// Delete removes the record of the image.
	Delete(id string) error

	// Wipe removes records of all images.
	Wipe() error

	// Lookup attempts to translate a name to an ID.  Most methods do this
	// implicitly.
	Lookup(name string) (string, error)

	// Images returns a slice enumerating the known images.
	Images() ([]Image, error)
}

type imageStore struct {
	lockfile Locker
	dir      string
	images   []Image
	byid     map[string]*Image
	byname   map[string]*Image
}

func (r *imageStore) Images() ([]Image, error) {
	return r.images, nil
}

func (r *imageStore) imagespath() string {
	return filepath.Join(r.dir, "images.json")
}

func (r *imageStore) datadir(id string) string {
	return filepath.Join(r.dir, id)
}

func (r *imageStore) datapath(id, key string) string {
	return filepath.Join(r.datadir(id), makeBigDataBaseName(key))
}

func (r *imageStore) Load() error {
	needSave := false
	rpath := r.imagespath()
	data, err := ioutil.ReadFile(rpath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	images := []Image{}
	ids := make(map[string]*Image)
	names := make(map[string]*Image)
	if err = json.Unmarshal(data, &images); len(data) == 0 || err == nil {
		for n, image := range images {
			ids[image.ID] = &images[n]
			for _, name := range image.Names {
				if conflict, ok := names[name]; ok {
					r.removeName(conflict, name)
					needSave = true
				}
				names[name] = &images[n]
			}
		}
	}
	r.images = images
	r.byid = ids
	r.byname = names
	if needSave {
		r.Touch()
		return r.Save()
	}
	return nil
}

func (r *imageStore) Save() error {
	rpath := r.imagespath()
	if err := os.MkdirAll(filepath.Dir(rpath), 0700); err != nil {
		return err
	}
	jdata, err := json.Marshal(&r.images)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(rpath, jdata, 0600)
}

func newImageStore(dir string) (ImageStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lockfile, err := GetLockfile(filepath.Join(dir, "images.lock"))
	if err != nil {
		return nil, err
	}
	lockfile.Lock()
	defer lockfile.Unlock()
	istore := imageStore{
		lockfile: lockfile,
		dir:      dir,
		images:   []Image{},
		byid:     make(map[string]*Image),
		byname:   make(map[string]*Image),
	}
	if err := istore.Load(); err != nil {
		return nil, err
	}
	return &istore, nil
}

func (r *imageStore) ClearFlag(id string, flag string) error {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if _, ok := r.byid[id]; !ok {
		return ErrImageUnknown
	}
	image := r.byid[id]
	delete(image.Flags, flag)
	return r.Save()
}

func (r *imageStore) SetFlag(id string, flag string, value interface{}) error {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if _, ok := r.byid[id]; !ok {
		return ErrImageUnknown
	}
	image := r.byid[id]
	image.Flags[flag] = value
	return r.Save()
}

func (r *imageStore) Create(id string, names []string, layer, metadata string) (image *Image, err error) {
	if id == "" {
		id = stringid.GenerateRandomID()
		_, idInUse := r.byid[id]
		for idInUse {
			id = stringid.GenerateRandomID()
			_, idInUse = r.byid[id]
		}
	}
	if _, idInUse := r.byid[id]; idInUse {
		return nil, ErrDuplicateID
	}
	for _, name := range names {
		if _, nameInUse := r.byname[name]; nameInUse {
			return nil, ErrDuplicateName
		}
	}
	if err == nil {
		newImage := Image{
			ID:           id,
			Names:        names,
			TopLayer:     layer,
			Metadata:     metadata,
			BigDataNames: []string{},
			BigDataSizes: make(map[string]int64),
			Flags:        make(map[string]interface{}),
		}
		r.images = append(r.images, newImage)
		image = &r.images[len(r.images)-1]
		r.byid[id] = image
		for _, name := range names {
			r.byname[name] = image
		}
		err = r.Save()
	}
	return image, err
}

func (r *imageStore) GetMetadata(id string) (string, error) {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if image, ok := r.byid[id]; ok {
		return image.Metadata, nil
	}
	return "", ErrImageUnknown
}

func (r *imageStore) SetMetadata(id, metadata string) error {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if image, ok := r.byid[id]; ok {
		image.Metadata = metadata
		return r.Save()
	}
	return ErrImageUnknown
}

func (r *imageStore) removeName(image *Image, name string) {
	newNames := []string{}
	for _, oldName := range image.Names {
		if oldName != name {
			newNames = append(newNames, oldName)
		}
	}
	image.Names = newNames
}

func (r *imageStore) SetNames(id string, names []string) error {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if image, ok := r.byid[id]; ok {
		for _, name := range image.Names {
			delete(r.byname, name)
		}
		for _, name := range names {
			if strings.Contains(name, "@") {
				name = name[:strings.Index(name, "@")]
			}
			if name == "" {
				continue
			}
			if otherImage, ok := r.byname[name]; ok {
				r.removeName(otherImage, name)
			}
			r.byname[name] = image
			nameWithID := name + "@" + id
			if otherImage, ok := r.byname[nameWithID]; ok {
				r.removeName(otherImage, nameWithID)
			}
			r.byname[nameWithID] = image
		}
		image.Names = names
		return r.Save()
	}
	return ErrImageUnknown
}

func (r *imageStore) Delete(id string) error {
	if image, ok := r.byname[id]; ok {
		id = image.ID
	}
	if _, ok := r.byid[id]; !ok {
		return ErrImageUnknown
	}
	if image, ok := r.byid[id]; ok {
		newImages := []Image{}
		for _, candidate := range r.images {
			if candidate.ID != id {
				newImages = append(newImages, candidate)
			}
		}
		delete(r.byid, image.ID)
		for _, name := range image.Names {
			delete(r.byname, name)
		}
		r.images = newImages
		if err := r.Save(); err != nil {
			return err
		}
		if err := os.RemoveAll(r.datadir(id)); err != nil {
			return err
		}
	}
	return nil
}

func (r *imageStore) Get(id string) (*Image, error) {
	if image, ok := r.byname[id]; ok {
		return image, nil
	}
	if image, ok := r.byid[id]; ok {
		return image, nil
	}
	return nil, ErrImageUnknown
}

func (r *imageStore) Lookup(name string) (id string, err error) {
	image, ok := r.byname[name]
	if !ok {
		image, ok = r.byid[name]
		if !ok {
			return "", ErrImageUnknown
		}
	}
	return image.ID, nil
}

func (r *imageStore) Exists(id string) bool {
	if _, ok := r.byname[id]; ok {
		return true
	}
	if _, ok := r.byid[id]; ok {
		return true
	}
	return false
}

func (r *imageStore) GetBigData(id, key string) ([]byte, error) {
	if img, ok := r.byname[id]; ok {
		id = img.ID
	}
	if _, ok := r.byid[id]; !ok {
		return nil, ErrImageUnknown
	}
	return ioutil.ReadFile(r.datapath(id, key))
}

func (r *imageStore) GetBigDataSize(id, key string) (int64, error) {
	if img, ok := r.byname[id]; ok {
		id = img.ID
	}
	if _, ok := r.byid[id]; !ok {
		return -1, ErrImageUnknown
	}
	if size, ok := r.byid[id].BigDataSizes[key]; ok {
		return size, nil
	}
	return -1, ErrSizeUnknown
}

func (r *imageStore) GetBigDataNames(id string) ([]string, error) {
	if img, ok := r.byname[id]; ok {
		id = img.ID
	}
	if _, ok := r.byid[id]; !ok {
		return nil, ErrImageUnknown
	}
	return r.byid[id].BigDataNames, nil
}

func (r *imageStore) SetBigData(id, key string, data []byte) error {
	if img, ok := r.byname[id]; ok {
		id = img.ID
	}
	if _, ok := r.byid[id]; !ok {
		return ErrImageUnknown
	}
	if err := os.MkdirAll(r.datadir(id), 0700); err != nil {
		return err
	}
	err := ioutils.AtomicWriteFile(r.datapath(id, key), data, 0600)
	if err == nil {
		add := true
		save := false
		oldSize, ok := r.byid[id].BigDataSizes[key]
		r.byid[id].BigDataSizes[key] = int64(len(data))
		if !ok || oldSize != r.byid[id].BigDataSizes[key] {
			save = true
		}
		for _, name := range r.byid[id].BigDataNames {
			if name == key {
				add = false
				break
			}
		}
		if add {
			r.byid[id].BigDataNames = append(r.byid[id].BigDataNames, key)
			save = true
		}
		if save {
			err = r.Save()
		}
	}
	return err
}

func (r *imageStore) Wipe() error {
	ids := []string{}
	for id := range r.byid {
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := r.Delete(id); err != nil {
			return err
		}
	}
	return nil
}

func (r *imageStore) Lock() {
	r.lockfile.Lock()
}

func (r *imageStore) Unlock() {
	r.lockfile.Unlock()
}

func (r *imageStore) Touch() error {
	return r.lockfile.Touch()
}

func (r *imageStore) Modified() (bool, error) {
	return r.lockfile.Modified()
}

func (r *imageStore) TouchedSince(when time.Time) bool {
	return r.lockfile.TouchedSince(when)
}
