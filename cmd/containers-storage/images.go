package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/mflag"
	digest "github.com/opencontainers/go-digest"
)

var (
	imagesQuiet = false
)

func images(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images, err := m.Images()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(images)
	} else {
		for _, image := range images {
			fmt.Printf("%s\n", image.ID)
			if imagesQuiet {
				continue
			}
			for _, name := range image.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			for _, digest := range image.Digests {
				fmt.Printf("\tdigest: %s\n", digest.String())
			}
			for _, name := range image.Repositories {
				fmt.Printf("\trepo: %s\n", name)
			}
			repos := sort.StringSlice{}
			for repo := range image.Tags {
				repos = append(repos, repo)
			}
			repos.Sort()
			for _, repo := range repos {
				for _, tag := range image.Tags[repo] {
					fmt.Printf("\ttag[%s]: %s\n", repo, tag)
				}
			}
			for _, name := range image.RepoTags {
				fmt.Printf("\trepotag: %s\n", name)
			}
			for _, name := range image.RepoDigests {
				fmt.Printf("\trepodigest: %s\n", name)
			}
			for _, name := range image.BigDataNames {
				fmt.Printf("\tdata: %s\n", name)
			}
		}
	}
	return 0
}

func imagesByDigest(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images := []*storage.Image{}
	for _, arg := range args {
		d := digest.Digest(arg)
		if err := d.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", arg, err)
			return 1
		}
		matched, err := m.ImagesByDigest(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		for _, match := range matched {
			images = append(images, match)
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(images)
	} else {
		for _, image := range images {
			fmt.Printf("%s\n", image.ID)
			if imagesQuiet {
				continue
			}
			for _, name := range image.Names {
				fmt.Printf("\tname: %s\n", name)
			}
			for _, name := range image.BigDataNames {
				fmt.Printf("\tdata: %s\n", name)
			}
		}
	}
	return 0
}

func imagesByRepository(flags *mflag.FlagSet, action string, m storage.Store, args []string) int {
	images := struct {
		Tags    map[string]map[string]string
		Digests map[string]map[digest.Digest]string
	}{
		Tags:    make(map[string]map[string]string),
		Digests: make(map[string]map[digest.Digest]string),
	}
	for _, repository := range args {
		tags, digests, err := m.ImagesByRepository(repository)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		if _, ok := images.Tags[repository]; !ok {
			images.Tags[repository] = make(map[string]string)
		}
		for tag, image := range tags {
			images.Tags[repository][tag] = image.ID
		}
		if _, ok := images.Digests[repository]; !ok {
			images.Digests[repository] = make(map[digest.Digest]string)
		}
		for digest, image := range digests {
			images.Digests[repository][digest] = image.ID
		}
	}
	if jsonOutput {
		json.NewEncoder(os.Stdout).Encode(images)
	} else {
		for _, repository := range args {
			fmt.Printf("%s:\n", repository)
			if len(images.Tags) > 0 {
				fmt.Printf("  tags:\n")
			}
			for tag, id := range images.Tags[repository] {
				fmt.Printf("    %s: %s\n", tag, id)
				if imagesQuiet {
					continue
				}
				image, err := m.Image(id)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					return 1
				}
				for _, name := range image.Names {
					fmt.Printf("      name: %s\n", name)
				}
				for _, repotag := range image.RepoTags {
					fmt.Printf("      repotag: %s\n", repotag)
				}
				for _, repodigest := range image.RepoDigests {
					fmt.Printf("      repodigest: %s\n", repodigest)
				}
			}
			if len(images.Digests) > 0 {
				fmt.Printf("  digests:\n")
			}
			for digest, id := range images.Digests[repository] {
				fmt.Printf("    %s: %s\n", digest.String(), id)
				if imagesQuiet {
					continue
				}
				image, err := m.Image(id)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					return 1
				}
				for _, name := range image.Names {
					fmt.Printf("      name: %s\n", name)
				}
				for _, repotag := range image.RepoTags {
					fmt.Printf("      repotag: %s\n", repotag)
				}
				for _, repodigest := range image.RepoDigests {
					fmt.Printf("      repotag: %s\n", repodigest)
				}
			}
		}
	}
	return 0
}

func init() {
	commands = append(commands, command{
		names:       []string{"images"},
		optionsHelp: "[options [...]]",
		usage:       "List images",
		action:      images,
		maxArgs:     0,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			flags.BoolVar(&imagesQuiet, []string{"-quiet", "q"}, imagesQuiet, "Only print IDs")
		},
	})
	commands = append(commands, command{
		names:       []string{"images-by-digest"},
		optionsHelp: "[options [...]] DIGEST",
		usage:       "List images by digest",
		action:      imagesByDigest,
		minArgs:     1,
		maxArgs:     1,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			flags.BoolVar(&imagesQuiet, []string{"-quiet", "q"}, imagesQuiet, "Only print IDs")
		},
	})
	commands = append(commands, command{
		names:       []string{"images-by-repository"},
		optionsHelp: "[options [...]] REPOSITORY",
		usage:       "List images by repository",
		action:      imagesByRepository,
		minArgs:     1,
		maxArgs:     1,
		addFlags: func(flags *mflag.FlagSet, cmd *command) {
			flags.BoolVar(&jsonOutput, []string{"-json", "j"}, jsonOutput, "Prefer JSON output")
			flags.BoolVar(&imagesQuiet, []string{"-quiet", "q"}, imagesQuiet, "Only print IDs")
		},
	})
}
