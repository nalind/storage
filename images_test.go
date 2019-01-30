package storage

import (
	"fmt"
	"sort"
	"testing"

	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func TestRecomputeNames(t *testing.T) {
	aDigest := digest.FromString("")
	aDigestString := aDigest.String()
	anotherDigest := digest.FromString("another digest")
	anotherDigestString := anotherDigest.String()
	aThirdDigest := digest.FromString("a third digest")
	aThirdDigestString := aThirdDigest.String()
	successes := []struct {
		// input values
		inputNames           []string
		inputImplicitDigests []digest.Digest
		// values that we expect to get back
		names        []string
		digests      []digest.Digest
		tags         map[string][]string
		repositories []string
		repotags     []string
		repodigests  []string
	}{
		{},
		{
			// names are not required
			inputImplicitDigests: []digest.Digest{anotherDigest, aDigest},
			digests:              []digest.Digest{aDigest, anotherDigest},
		},
		{
			// repository names get normalized
			inputNames:   []string{"foo"},
			names:        []string{"foo"},
			repositories: []string{"docker.io/library/foo"},
		},
		{
			// repository names get normalized
			inputNames:   []string{"bar"},
			names:        []string{"bar"},
			repositories: []string{"docker.io/library/bar"},
		},
		{
			// repository names get normalized
			inputNames:   []string{"foo", "bar"},
			names:        []string{"foo", "bar"},
			repositories: []string{"docker.io/library/foo", "docker.io/library/bar"},
		},
		{
			// repository names get normalized, make sure we handle tags right
			inputNames: []string{"foo:owl", "bar:baz"},
			names:      []string{"foo:owl", "bar:baz"},
			tags: map[string][]string{
				"docker.io/library/foo": []string{"owl"},
				"docker.io/library/bar": []string{"baz"},
			},
			repositories: []string{"docker.io/library/foo", "docker.io/library/bar"},
			repotags:     []string{"docker.io/library/foo:owl", "docker.io/library/bar:baz"},
		},
		{
			// repository names get normalized, make sure we handle tags right
			inputNames: []string{"foo:owl", "foo:fish", "bar:baz"},
			names:      []string{"foo:owl", "foo:fish", "bar:baz"},
			tags: map[string][]string{
				"docker.io/library/foo": []string{"fish", "owl"},
				"docker.io/library/bar": []string{"baz"},
			},
			repositories: []string{"docker.io/library/foo", "docker.io/library/bar"},
			repotags:     []string{"docker.io/library/foo:owl", "docker.io/library/bar:baz", "docker.io/library/foo:fish"},
		},
		{
			// repository names get normalized, make sure we preserve and understand digested names
			inputNames: []string{"foo:owl", "foo:fish", "foo@" + aDigestString, "bar:baz", "bar@" + anotherDigestString},
			names:      []string{"foo:owl", "foo:fish", "bar:baz", "foo@" + aDigestString, "bar@" + anotherDigestString},
			tags: map[string][]string{
				"docker.io/library/foo": []string{"fish", "owl"},
				"docker.io/library/bar": []string{"baz"},
			},
			repositories: []string{"docker.io/library/foo", "docker.io/library/bar"},
			repotags:     []string{"docker.io/library/foo:owl", "docker.io/library/bar:baz", "docker.io/library/foo:fish"},
			repodigests:  []string{"docker.io/library/foo@" + aDigestString, "docker.io/library/bar@" + anotherDigestString},
		},
		{
			// make sure we apply implicit digests to repository names to add to the repodigests list
			inputNames:           []string{"foo:owl", "foo:fish", "foo@" + aDigestString},
			inputImplicitDigests: []digest.Digest{anotherDigest},
			names:                []string{"foo:owl", "foo:fish", "foo@" + aDigestString},
			digests:              []digest.Digest{anotherDigest},
			tags: map[string][]string{
				"docker.io/library/foo": []string{"fish", "owl"},
			},
			repositories: []string{"docker.io/library/foo"},
			repotags:     []string{"docker.io/library/foo:owl", "docker.io/library/foo:fish"},
			repodigests:  []string{"docker.io/library/foo@" + aDigestString, "docker.io/library/foo@" + anotherDigestString},
		},
		{
			// make sure we apply implicit digests to repository names to add to the repodigests list
			inputNames:           []string{"foo:owl", "foo:fish", "foo@" + aDigestString, "bar@" + anotherDigestString},
			inputImplicitDigests: []digest.Digest{anotherDigest, aThirdDigest},
			names:                []string{"foo:owl", "foo:fish", "foo@" + aDigestString, "bar@" + anotherDigestString},
			digests:              []digest.Digest{anotherDigest, aThirdDigest},
			tags: map[string][]string{
				"docker.io/library/foo": []string{"fish", "owl"},
			},
			repositories: []string{"docker.io/library/foo", "docker.io/library/bar"},
			repotags:     []string{"docker.io/library/foo:owl", "docker.io/library/foo:fish"},
			repodigests: []string{
				"docker.io/library/foo@" + aDigestString,
				"docker.io/library/foo@" + anotherDigestString,
				"docker.io/library/foo@" + aThirdDigestString,
				"docker.io/library/bar@" + anotherDigestString,
				"docker.io/library/bar@" + aThirdDigestString,
			},
		},
	}
	failures := []struct {
		inputNames           []string
		inputImplicitDigests []digest.Digest
	}{
		{inputNames: []string{""}},
		{inputNames: []string{"not allowed to include whitespace"}},
		{inputImplicitDigests: []digest.Digest{digest.Digest("sha256:no-way-this-is-valid")}},
	}
	stringsFromDigests := func(ds []digest.Digest) []string {
		var s []string
		for _, d := range ds {
			s = append(s, d.String())
		}
		return s
	}
	compareStringLists := func(t *testing.T, expected, actual []string, context string) {
		if len(actual) != len(expected) {
			assert.Exactly(t, len(expected), len(actual), "expected list %v, got list %v at %s", expected, actual, context)
		}
		e := sort.StringSlice(expected)
		sort.Strings(e)
		a := sort.StringSlice(actual)
		sort.Strings(a)
		for i := range e {
			assert.Exactly(t, e[i], a[i], "expected list %v, got list %v at %s", a, e, context)
		}
	}
	compareStringListMaps := func(t *testing.T, expected, actual map[string][]string) {
		var ak, ek sort.StringSlice
		for k := range expected {
			ek = append(ek, k)
		}
		sort.Strings(ek)
		for k := range actual {
			ak = append(ak, k)
		}
		sort.Strings(ak)
		compareStringLists(t, ek, ak, "repository list while comparing tag maps")
		for _, k := range ek {
			compareStringLists(t, expected[k], actual[k], fmt.Sprintf("tag list for repository %q", k))
		}
	}
	for _, this := range failures {
		_, _, _, _, _, _, err := recomputeImageNames(this.inputNames, this.inputImplicitDigests)
		assert.NotNil(t, err, "err should be set for %#v,%#v, but was not", this.inputNames, this.inputImplicitDigests)
	}
	for i, this := range successes {
		names, digests, repositories, tags, repotags, repodigests, err := recomputeImageNames(this.inputNames, this.inputImplicitDigests)
		assert.Nil(t, err, "err should be nothing, is %v", err)
		compareStringLists(t, this.names, names, fmt.Sprintf("valid names list for %d:%#v,%#v", i, this.inputNames, this.inputImplicitDigests))
		compareStringLists(t, stringsFromDigests(this.digests), stringsFromDigests(digests), fmt.Sprintf("digests list for %d:%#v,%#v", i, this.inputNames, this.inputImplicitDigests))
		compareStringListMaps(t, this.tags, tags)
		compareStringLists(t, this.repositories, repositories, fmt.Sprintf("repositories list for %d:%#v,%#v", i, this.inputNames, this.inputImplicitDigests))
		compareStringLists(t, this.repotags, repotags, fmt.Sprintf("repotags list for %d:%#v,%#v", i, this.inputNames, this.inputImplicitDigests))
		compareStringLists(t, this.repodigests, repodigests, fmt.Sprintf("repodigest list for %d:%#v,%#v", i, this.inputNames, this.inputImplicitDigests))
	}
}
