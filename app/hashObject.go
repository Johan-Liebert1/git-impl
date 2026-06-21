package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"os"
)

func badArgsPanic(argLen int) {
	if len(os.Args) < argLen {
		panic("usage ./exe hashObject [-w] <file_name>")
	}
}

// Accepts buffer and the hash of the buffer
// Writes the equivalent file to .git/objects/xx/xx...
func commitToDisk(repoPath string, bytes bytes.Buffer, hexHash string) {
	dirPath := fmt.Sprintf("%s/.git/objects/%s", repoPath, hexHash[:2])

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		panic(err)
	}

	hashFile, err := os.Create(fmt.Sprintf("%s/%s", dirPath, hexHash[2:]))

	if err != nil {
		panic(err)
	}

	zlibWriter := zlib.NewWriter(hashFile)

	n, err := zlibWriter.Write(bytes.Bytes())

	if err != nil {
		panic(err)
	}

	if n != len(bytes.Bytes()) {
		panic("Could not write entire thing")
	}

	if err := zlibWriter.Flush(); err != nil {
		fmt.Printf("flushing zlibWriter failed: %+v\n", err)
		os.Exit(1)
	}

	// The final checksum is written by this
	if err := zlibWriter.Close(); err != nil {
		fmt.Printf("closing zlibWriter failed: %+v\n", err)
		os.Exit(1)
	}
}

// Reads a file, converts it to a git blob
// Computes it's sha1 hash
// Saves it in .git/objects/xx/xxxx...
// Returns the sha1 hash
func hashFile(fileName string, writeToFile bool, printOut bool) []byte {
	file, err := os.ReadFile(fileName)

	if err != nil {
		fmt.Printf("Failed to read file '%s': %+v\n", fileName, err)
		os.Exit(1)
	}

	// Header is of the form [blob,commit] <len_contents>\0
	var bytes bytes.Buffer
	bytes.WriteString(fmt.Sprintf("blob %d\x00", len(file)))
	bytes.Write(file)

	// Although the object file is stored with zlib compression,
	// the SHA-1 hash needs to be computed over the "uncompressed"
	// contents of the file, not the compressed version.
	sha1Hash := sha1.New()
	sha1Hash.Write(bytes.Bytes())
	hashBytes := sha1Hash.Sum(nil)

	hexHash := fmt.Sprintf("%x", hashBytes)

	if !writeToFile {
		if printOut {
			fmt.Println(hexHash)
		}

		return hashBytes
	}

	commitToDisk(".", bytes, hexHash)

	if printOut {
		fmt.Println(hexHash)
	}

	return hashBytes
}

// git hash-object [-w] <file_name>
func hashObject() {
	badArgsPanic(3)

	var (
		writeToFile = false
		fileName    = ""
	)

	if os.Args[2] == "-w" {
		writeToFile = true
		badArgsPanic(4)
		fileName = os.Args[3]
	} else {
		fileName = os.Args[2]
	}

	hashFile(fileName, writeToFile, true)
}
