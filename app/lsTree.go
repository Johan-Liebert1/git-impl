package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

func lsTreeCmd() {
	nameOnly := os.Args[2] == "--name-only"

	treeSha := ""

	if nameOnly {
		treeSha = os.Args[3]
	} else {
		treeSha = os.Args[2]
	}

	lsTree(os.Stdout, treeSha, nameOnly)
}

func lsTree(writer io.Writer, treeSha string, nameOnly bool) {
	file := readGitObject(treeSha)
	decompressed := decompressGitObj(file)

	// The format of a tree object file looks like this (after Zlib decompression)
	// There are no new lines in the actual tree objects
	//
	//   tree <size>\0
	//   <mode> <name>\0<20_byte_sha (not hex)>
	//   <mode> <name>\0<20_byte_sha (not hex)>

	// parse the "tree <size>\0" header
	nullIdx := findFirstNull(decompressed, 0)

	if string(decompressed[:4]) != "tree" {
		fmt.Printf("%s is not a valid tree object. Does not start with 'tree' starts with '%s'\n", treeSha, string(decompressed[:4]))
		os.Exit(1)
	}

	// 6 because there is a space after tree
	probSize := string(decompressed[5:nullIdx])

	// Size is basically how many bytes there are to read
	// See loop
	size, err := strconv.Atoi(probSize)

	if err != nil {
		fmt.Printf("Invalid size for tree %s: %s\n", treeSha, probSize)
		os.Exit(1)
	}

	parsedBytes := 0 
	currIdx := nullIdx

	for parsedBytes < size {
		// <mode> <name>\0<20_byte_sha (not hex)>
		// mode is a number

		spaceIdx := findFirstChar(decompressed, currIdx+1, ' ')

		if spaceIdx == -1 {
			// Something went wrong here
			fmt.Printf("Invalid tree object. Failed to parse")
			os.Exit(1)
		}

		probMode := string(decompressed[currIdx+1 : spaceIdx])

		mode, err := strconv.Atoi(probMode)

		if err != nil {
			fmt.Printf("Invalid mode for tree %s: %s\n", treeSha, probMode)
			os.Exit(1)
		}

		nameEnd := findFirstNull(decompressed, spaceIdx+1)
		name := string(decompressed[spaceIdx+1:nameEnd])

		// Now we read the 20 byte sha
		end := nameEnd+1+20
		sha := fmt.Sprintf("%x", decompressed[nameEnd+1:end])

		parsedBytes += end - 1 - currIdx
		currIdx = end - 1

		if !nameOnly {
			fmt.Fprintf(writer, "%d %s %s\n", mode, sha, name)
		} else {
			fmt.Fprintf(writer, "%s\n", name)
		}
	}
}
