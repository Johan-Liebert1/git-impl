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

	file, err := os.ReadFile(fileName)

	if err != nil {
		panic(err)
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
		fmt.Println(hexHash)
		return
	}

	dirPath := fmt.Sprintf("./.git/objects/%s", hexHash[:2])

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

	zlibWriter.Flush()

	// The final checksum is written by this
	zlibWriter.Close()

	fmt.Println(hexHash)
}
