package main

import (
	"regexp"
	"testing"
)

// The operator site is bilingual: every key must exist in both maps, and every
// key the skeleton references must exist at all. This is the enforcement behind
// the "agent edits = site edits" contract — prose drift fails the build.

func TestSiteStringKeysParity(t *testing.T) {
	for k := range siteEN {
		if _, ok := siteRU[k]; !ok {
			t.Errorf("key %q exists in siteEN but is missing in siteRU", k)
		}
	}
	for k := range siteRU {
		if _, ok := siteEN[k]; !ok {
			t.Errorf("key %q exists in siteRU but is missing in siteEN", k)
		}
	}
}

func TestSkeletonReferencesExistingKeys(t *testing.T) {
	re := regexp.MustCompile(`\{\{\$?\.T\.([A-Za-z0-9_]+)\}\}`)
	for _, m := range re.FindAllStringSubmatch(siteSkeleton, -1) {
		if _, ok := siteEN[m[1]]; !ok {
			t.Errorf("skeleton references key %q not present in siteEN", m[1])
		}
	}
}
