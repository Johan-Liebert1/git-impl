package main

import (
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strconv"
)

func gitObjectSize(buffer []byte) int {
	spaceIdx := findFirstChar(buffer, 0, ' ')
	
	if spaceIdx == -1 {
		fmt.Printf("Bad git object. Could not find space byte to get size")
		os.Exit(1)
	}

	nullIdx := findFirstNull(buffer, 0)

	if nullIdx == -1 {
		fmt.Printf("Bad git object. Could not find null byte to get size")
		os.Exit(1)
	}

	probSize := string(buffer[spaceIdx+1:nullIdx])
	i, err := strconv.ParseInt(probSize, 10, 64)

	if err != nil {
		fmt.Printf("Bad size: '%s'\n", probSize)
		os.Exit(1)
	}

	return int(i)
}

func gitObjectExists(gitRepoDir string, objSha string) bool {
	filePath := fmt.Sprintf("%s/%s", objSha[:2], objSha[2:])
	fullFilePath := fmt.Sprintf("%s/.git/objects/%s", gitRepoDir, filePath)

	_, err := os.Stat(fullFilePath)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		fmt.Printf("Failed to check if file '%s' exists: %+v\n", fullFilePath, err)
		os.Exit(1)
	}

	return false
}

func gitObjType(uncompressedData []byte) string {
	if string(uncompressedData[:4]) == "blob" {
		return "blob"
	}

	if string(uncompressedData[:6]) == "commit" {
		return "commit"
	}

	if string(uncompressedData[:4]) == "tree" {
		return "tree"
	}

	fmt.Printf("unknown git object type: %s\n", string(uncompressedData[:10]))
	os.Exit(1)

	return ""
}

func readGitObject(gitRepoDir string, objSha string) *os.File {
	filePath := fmt.Sprintf("%s/%s", objSha[:2], objSha[2:])
	file, err := os.Open(fmt.Sprintf("%s/.git/objects/%s", gitRepoDir, filePath))

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

func computeSha1Hash(buf []byte) []byte {
	hasher := sha1.New()
	hasher.Write(buf)
	return hasher.Sum(nil)
}
