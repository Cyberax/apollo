package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func walkReferences(doc map[interface{}]interface{}, referencedFiles *map[string]string) {
	for k, v := range doc {
		subdoc, ok := v.(map[interface{}]interface{})
		if ok {
			walkReferences(subdoc, referencedFiles)
			continue
		}
		subarr, ok := v.([]interface{})
		if ok {
			for _, arrElem := range subarr {
				subArrMap, ok := arrElem.(map[interface{}]interface{})
				if ok {
					walkReferences(subArrMap, referencedFiles)
				}
			}
			continue
		}

		if k == "$ref" {
			// Localize the reference and add it to our list
			refVal := v.(string)
			splitRef := strings.SplitN(refVal, "#", 2)
			if len(splitRef) == 1 || splitRef[0] == "" {
				continue
			}

			(*referencedFiles)[splitRef[0]] = splitRef[0]
			refVal = splitRef[1]
			// Patch the reference
			doc[k] = "#" + refVal
		}
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprint(os.Stderr, "Usage: mergeyaml input.yaml output.yaml")
		os.Exit(1)
	}
	inFile := os.Args[1]
	outFile := os.Args[2]

	var referencedFiles = make(map[string]string)
	var visitedFiles = make(map[string]map[interface{}]interface{})

	fileName := path.Base(inFile)
	dir := path.Dir(inFile)

	referencedFiles[fileName] = fileName

	// Now walk the files until everything is visited
	for ; len(visitedFiles) < len(referencedFiles) ; {
		for fl := range referencedFiles {
			if _, ok := visitedFiles[fl]; ok {
				continue
			}

			flName := fl
			if flName[0] != '/' {
				flName = path.Join(dir, flName)
			}

			bytes, err := ioutil.ReadFile(flName)
			if err != nil {
				panic(err)
			}

			doc := map[interface{}]interface{}{}

			err = yaml.Unmarshal(bytes, &doc)
			if err != nil {
				panic(err)
			}

			// Walk through the document, and parse references
			walkReferences(doc, &referencedFiles)

			visitedFiles[fl] = doc
		}
	}

	// Now merge everything together
	var finalDoc = make(map[interface{}]interface{})
	for name, fileRoot := range visitedFiles {
		for rootKey, rootVal := range fileRoot {
			curVal, ok := finalDoc[rootKey]
			if !ok {
				finalDoc[rootKey] = rootVal
				continue
			}

			curMapVal, ok := curVal.(map[interface{}]interface{})
			if !ok {
				fmt.Fprintf(os.Stderr, "Key %s is not a map\n", rootKey)
				os.Exit(1)
			}

			mapRootVal, ok := rootVal.(map[interface{}]interface{})
			if !ok {
				fmt.Fprintf(os.Stderr, "Key %s in %s is not a map\n", rootKey, name)
				os.Exit(1)
			}

			// Now merge the map.
			for k, v := range mapRootVal {
				curMapVal[k] = v
			}
		}
	}

	delete(finalDoc, "x-paths")

	out, err := yaml.Marshal(finalDoc)
	if err != nil {
		panic(err)
	}

	ioutil.WriteFile(outFile, out, 0660)
}
