package main

import (
	"regexp"
)

var versionLineRegex = regexp.MustCompile(`^##[ \t]+`)
var versionLineDetailsRegex = regexp.MustCompile(`^##[ \t]+\[?(Unreleased|[^\s\]]+)]?`)

type Version struct {
	Label           string
	StartLineNumber int
	EndLineNumber   int
}

func NewVersions(text []string) []*Version {
	var versions []*Version
	for idx, line := range text {
		if versionLineRegex.MatchString(line) {
			versions = append(
				versions,
				&Version{
					StartLineNumber: idx,
				},
			)
		}
	}

	for idx, version := range versions {
		if idx == len(versions) - 1 {
			version.EndLineNumber = len(text)
			continue
		}

		version.EndLineNumber = versions[idx + 1].StartLineNumber - 1
	}

	if len(versions) == 0 {
		return nil
	}

	for _, version := range versions {
		// TODO: probably more efficient to run this regex on all the lines at once
		version.Label = versionLineDetailsRegex.FindAllStringSubmatch(text[version.StartLineNumber], -1)[0][1]
	}

	return versions
}
