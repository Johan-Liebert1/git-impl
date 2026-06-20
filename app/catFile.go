package main

import (
	"fmt"
	"os"
)

func catFile() {
	_ = os.Args[2]

	fileName := os.Args[3]

	file := readGitObject(fileName)

	read := decompressGitObj(file)

	// The file contains a header and the contents of the blob object, compressed using Zlib.
	//
	// blob <size>\0<content>
	//
	// <size> is the size of the content (in bytes)
	//
	// \0 is a null byte
	//
	// <content> is the actual content of the file
	//
	// For example, if the contents of a file are hello world, the blob object file would look like this (after Zlib decompression):
	//
	// blob 11\0hello world

	// Find the null byte

	nullIdx := -1

	for i, b := range read {
		if b == 0 {
			nullIdx = i
			break
		}
	}

	// TODO: Bad bad
	// Verify if the header matches this but eh...
	fmt.Print(string(read[nullIdx+1:]))
}
