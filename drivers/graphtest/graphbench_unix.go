//go:build linux || freebsd

package graphtest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	graphdriver "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/stringid"
)

// DriverBenchExists benchmarks calls to exist
func DriverBenchExists(b *testing.B, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		if !driver.Exists(base) {
			b.Fatal("Newly created image doesn't exist")
		}
	}
}

// DriverBenchGetEmpty benchmarks calls to get on an empty layer
func DriverBenchGetEmpty(b *testing.B, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)

	base := stringid.GenerateRandomID()

	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		_, err := driver.Get(base, graphdriver.MountOpts{})
		b.StopTimer()
		if err != nil {
			b.Fatalf("Error getting mount: %s", err)
		}
		if err := driver.Put(base); err != nil {
			b.Fatalf("Error putting mount: %s", err)
		}
		b.StartTimer()
	}
}

// DriverBenchDiffBase benchmarks calls to diff on a root layer
func DriverBenchDiffBase(b *testing.B, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)

	base := stringid.GenerateRandomID()
	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	if err := addFiles(driver, base, 3); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		arch, err := driver.Diff(base, nil, "", nil, "")
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(io.Discard, arch)
		if err != nil {
			b.Fatalf("Error copying archive: %s", err)
		}
		arch.Close()
	}
}

// DriverBenchDiffN benchmarks calls to diff on two layers with
// a provided number of files on the lower and upper layers.
func DriverBenchDiffN(b *testing.B, bottom, top int, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)
	base := stringid.GenerateRandomID()
	upper := stringid.GenerateRandomID()
	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	if err := addManyFiles(driver, base, bottom, 3); err != nil {
		b.Fatal(err)
	}

	if err := driver.Create(upper, base, nil); err != nil {
		b.Fatal(err)
	}

	if err := addManyFiles(driver, upper, top, 6); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		arch, err := driver.Diff(upper, nil, "", nil, "")
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(io.Discard, arch)
		if err != nil {
			b.Fatalf("Error copying archive: %s", err)
		}
		arch.Close()
	}
}

// DriverBenchDiffApplyN benchmarks calls to diff and apply together
func DriverBenchDiffApplyN(b *testing.B, fileCount int, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)
	base := stringid.GenerateRandomID()
	upper := stringid.GenerateRandomID()
	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	if err := addManyFiles(driver, base, fileCount, 3); err != nil {
		b.Fatal(err)
	}

	if err := driver.Create(upper, base, nil); err != nil {
		b.Fatal(err)
	}

	if err := addManyFiles(driver, upper, fileCount, 6); err != nil {
		b.Fatal(err)
	}
	diffSize, err := driver.DiffSize(upper, nil, "", nil, "")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.StopTimer()
	for range b.N {
		diff := stringid.GenerateRandomID()
		if err := driver.Create(diff, base, nil); err != nil {
			b.Fatal(err)
		}

		if err := checkManyFiles(driver, diff, fileCount, 3); err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		arch, err := driver.Diff(upper, nil, "", nil, "")
		if err != nil {
			b.Fatal(err)
		}

		applyDiffSize, err := driver.ApplyDiff(diff, "", graphdriver.ApplyDiffOpts{})
		if err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		arch.Close()

		if applyDiffSize != diffSize {
			b.Fatalf("Apply diff size different, got %d, expected %d\n", applyDiffSize, diffSize)
		}
		if err := checkManyFiles(driver, diff, fileCount, 6); err != nil {
			b.Fatal(err)
		}
	}
}

// DriverBenchDeepLayerDiff benchmarks calls to diff on top of a given number of layers.
func DriverBenchDeepLayerDiff(b *testing.B, layerCount int, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)

	base := stringid.GenerateRandomID()
	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	if err := addFiles(driver, base, 50); err != nil {
		b.Fatal(err)
	}

	topLayer, err := addManyLayers(b, driver, base, layerCount)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		arch, err := driver.Diff(topLayer, nil, "", nil, "")
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(io.Discard, arch)
		if err != nil {
			b.Fatalf("Error copying archive: %s", err)
		}
		arch.Close()
	}
}

// DriverBenchDeepLayerRead benchmarks calls to read a file under a given number of layers.
func DriverBenchDeepLayerRead(b *testing.B, layerCount int, drivername string, driveroptions ...string) {
	driver := GetDriver(b, drivername, driveroptions...)

	base := stringid.GenerateRandomID()
	if err := driver.Create(base, "", nil); err != nil {
		b.Fatal(err)
	}

	content := []byte("test content")
	if err := addFile(driver, base, "testfile.txt", content); err != nil {
		b.Fatal(err)
	}

	topLayer, err := addManyLayers(b, driver, base, layerCount)
	if err != nil {
		b.Fatal(err)
	}

	root, err := driver.Get(topLayer, graphdriver.MountOpts{})
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := driver.Put(topLayer); err != nil {
			b.Fatal(err)
		}
	}()

	b.ResetTimer()
	for range b.N {

		// Read content
		c, err := os.ReadFile(filepath.Join(root, "testfile.txt"))
		if err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		if !bytes.Equal(c, content) {
			b.Fatalf("Wrong content in file %v, expected %v", c, content)
		}
		b.StartTimer()
	}
}
