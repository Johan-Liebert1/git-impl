package main

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
)

func readGitObject(objSha string) *os.File {
	filePath := fmt.Sprintf("%s/%s", objSha[:2], objSha[2:])
	file, err := os.Open(fmt.Sprintf("./.git/objects/%s", filePath))

	if err != nil {
		panic(fmt.Sprintf("readGitObject: %s: Err: %+v", objSha, err))
	}

	return file
}

func decompressGitObj(file *os.File) []byte {
	reader, err := zlib.NewReader(file)

	if err != nil {
		panic(err)
	}

	read, err := io.ReadAll(reader)

	if err != nil {
		panic(fmt.Sprintf("decompressGitObj: Err: %+v", err))
	}

	return read
}

func findFirstNull(slice []byte, start int) int {
	return findFirstChar(slice, start, 0)
}


func findFirstChar(slice []byte, start int, char byte) int {
	idx := -1

	for i := start; i < len(slice); i++ {
		if slice[i] == char {
			idx = i
			break
		}
	}

	return idx
}
