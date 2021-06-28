package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

var linkLinesRegex = regexp.MustCompile(`(?m)^\s+(?:^\[[^]]+]:[ \t]+.*$\s+)+`)
var multipleEmptyLines = regexp.MustCompile(`(?m)^\s+^\s*`)

func main() {
	listVersionsFlag := flag.Bool("list", false, "lists all versions")
	releaseFlag := flag.String("release", "", "version to release")
	listOutputFlag := flag.String("list-output", "raw", "how to render the list. enum: 'raw' or 'markdown'")
	getVersionFlag := flag.String("get", "", "get specific version or versions within closed interval e.g. 0.1.1 or 0.1.1,0.1.2")
	getOutputFlag := flag.String("get-output", "raw", "how to render version output. enum: 'raw' or 'merged'")
	flag.Parse()

	if strings.Contains(*getOutputFlag, ",") || !strings.Contains("raw,merged", *getOutputFlag) {
		fmt.Printf("-get-output: unknown value (%s), acceptable value are: 'raw' or 'merged'\n", *getOutputFlag)
		os.Exit(1)
	}

	if strings.Contains(*listOutputFlag, ",") || !strings.Contains("raw,markdown", *listOutputFlag) {
		fmt.Printf("-list-output: unknown value (%s), acceptable value are: 'raw' or 'markdown'\n", *listOutputFlag)
		os.Exit(1)
	}

	// TODO this could be a read from anywhere, doesn't need to be the file system
	fileContents, err := ioutil.ReadFile("./CHANGELOG.md")
	if err != nil {
		log.Fatalf("failed to read file")
	}

	rawFileContents := fileContents

	linksText := strings.TrimSpace(
		string(
			bytes.Join(
				linkLinesRegex.FindAll(fileContents, -1),
				[]byte("\n"),
			),
		),
	)
	fileContents = linkLinesRegex.ReplaceAll(fileContents, []byte("\n"))

	if len(linksText) == 0 {
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(fileContents))

	scanner.Split(bufio.ScanLines)
	var text []string
	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	versions := NewVersions(text)
	if len(versions) == 0 {
		fmt.Println("No versions found. Nothing to do here!")
		os.Exit(1)
	}

	if len(*releaseFlag) > 0 {
		newVersion := *releaseFlag
		oldVersion := versions[1].Label

		output := string(rawFileContents)

		output = regexp.MustCompile("(\\[Unreleased]:)(.*)(/v.*HEAD)").ReplaceAllString(
			output,
			"$1$2/v" + newVersion + "...HEAD\n[" + newVersion + "]:$2/v" + oldVersion + "..." + newVersion,
		)
		output = regexp.MustCompile("## \\[Unreleased]").ReplaceAllString(
			output,
			`## [Unreleased]

## [` + newVersion + "] - " + time.Now().Format("2006-01-02"))

		fmt.Println(output)
		return
	}

	if *listVersionsFlag {
		if len(*listOutputFlag) == 0 {
			*listOutputFlag = "markdown"
		}

		switch *listOutputFlag {
		case "raw":
			for _, version := range versions {
				fmt.Println(version.Label)
			}
		case "markdown":
			for _, version := range versions {
				fmt.Println(text[version.StartLineNumber])
			}
		default:
			impossible()
		}

		return
	}

	if len(*getVersionFlag) == 0 {
		*getVersionFlag = "Unreleased"
	}

	requestedVersionLabels := strings.SplitN(*getVersionFlag, ",", -1)
	if len(requestedVersionLabels) > 2 {
		fmt.Println("Can only get a single version or a range delimited by, e.g. 0.1.1 or 0.1.1,0.1.2")
		os.Exit(1)
	}

	contentStartLineNumber, contentEndLineNumber, requestedVersions := selectRequestedVersions(requestedVersionLabels, text, versions)

	switch *getOutputFlag {
	case "raw":
		// Includes version label lines
		fmt.Println(
			strings.TrimSpace(
				strings.Join(
					text[contentStartLineNumber:contentEndLineNumber+1],
					"\n",
				),
			),
		)
		fmt.Println()
		fmt.Println(linksText)
	case "merged":
		contentTypes := mergedContentTypes(requestedVersions, text)

		for sectionType, sectionContent := range contentTypes {
			if len(sectionContent) == 0 {
				continue
			}

			fmt.Printf("## %s\n", sectionType)
			sectionContentText := strings.Join(sectionContent, "\n")
			fmt.Println(sectionContentText)
		}
		fmt.Println()
		// TODO: maybe just print the links that are present in the printed text
		//  better yet we should inline the links
		fmt.Println(linksText)
		return
	default:
		impossible()
	}
}

func mergedContentTypes(requestedVersions []*Version, text []string) map[string][]string {
	// TODO: quite naive, doesn't do any sorting on release date or version
	contentTypes := map[string][]string{
		"Added":      {},
		"Changed":    {},
		"Deprecated": {},
		"Removed":    {},
		"Fixed":      {},
		"Security":   {},
	}

	for _, version := range requestedVersions {
		// Excludes version label line
		versionContentText := text[version.StartLineNumber+1 : version.EndLineNumber+1]
		versionContentTypes := NewContentTypes(versionContentText)

		// TODO: isn't this similar to how it's done with versions ? Maybe dedupe
		// Content type should just have start and finish

		for idx, contentType := range versionContentTypes {
			startLineNumber := contentType.LineNumber
			var endLineNumber int
			if idx == len(versionContentTypes)-1 {
				endLineNumber = len(versionContentText) - 1
			} else {
				endLineNumber = versionContentTypes[idx+1].LineNumber
			}

			// Exclude type label line
			content := versionContentText[startLineNumber+1 : endLineNumber+1]

			// Append to version content type to all content types
			contentTypes[contentType.Label] = append(
				contentTypes[contentType.Label],
				strings.TrimSpace(strings.Join(content, "\n")),
			)
		}

	}
	return contentTypes
}

func selectRequestedVersions(
	requestedVersionLabels []string,
	text []string,
	versions []*Version,
) (int, int, []*Version) {
	var contentStartLineNumber, contentEndLineNumber int
	var requestedVersions = make([]*Version, len(requestedVersionLabels))
	// Initialise to extreme values
	contentStartLineNumber = len(text) // will be minimised depending on requested versions
	contentEndLineNumber = 0           // will be maximised depending on requested versions

	for idx, requestedVersionLabel := range requestedVersionLabels {
		var requestedVersion *Version
		for _, version := range versions {
			if requestedVersionLabel == version.Label {
				requestedVersion = version
				break
			}
		}

		if requestedVersion == nil {
			fmt.Printf("Requested version (%s) not found\n", requestedVersionLabel)
			os.Exit(1)
		}

		requestedVersions[idx] = requestedVersion

		// TODO: this should move into a separate function
		contentStartLineNumber = min(requestedVersion.StartLineNumber, contentStartLineNumber)
		contentEndLineNumber = max(requestedVersion.EndLineNumber, contentEndLineNumber)
	}

	return contentStartLineNumber, contentEndLineNumber, requestedVersions
}

func impossible() {
	fmt.Println("it should be impossible to get here. something went terrible wrong!")
	os.Exit(9000)
}
