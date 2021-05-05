package main

import (
	"regexp"
	"strings"
)

var contentTypeRegex = regexp.MustCompile("^###[ \\t]+(.*)")

func NewContentTypes(versionContentText []string) []*ContentType {
	var versionContentTypes []*ContentType
	for lineNumber, versionContentLine := range versionContentText {
		matches := contentTypeRegex.FindAllStringSubmatch(versionContentLine, -1)
		if len(matches) == 0 {
			continue
		}
		versionContentTypes = append(versionContentTypes, &ContentType{
			Label:      strings.Title(strings.TrimSpace(matches[0][1])), // Normalise content type
			LineNumber: lineNumber,
		})
	}

	return versionContentTypes
}

type ContentType struct {
	Label      string
	LineNumber int
}
