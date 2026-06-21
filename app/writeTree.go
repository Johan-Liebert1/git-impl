package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// computes the hash of the buffer
// Writes the tree object to .git/objects/xx/xx...

func commitObject(repoPath string, buffer *bytes.Buffer, objName string) []byte {
	// FIXME: This is extremely wasteful...
	newBuffer := bytes.Buffer{}

	newBuffer.WriteString(objName)
	newBuffer.WriteByte(' ')
	newBuffer.WriteString(fmt.Sprintf("%d", len(buffer.Bytes())))
	newBuffer.WriteByte(0)
	newBuffer.Write(buffer.Bytes())

	// compute the hash of new buffer and write it to disk
	hash := computeSha1Hash(newBuffer.Bytes())

	commitToDisk(repoPath, newBuffer, fmt.Sprintf("%x", hash))

	return hash
}

// The format of a tree object file looks like this (after Zlib decompression)
// There are no new lines in the actual tree objects
//
//   tree <size>\0
//   <mode> <name>\0<20_byte_sha (not hex)>
//   <mode> <name>\0<20_byte_sha (not hex)>
// The file begins with tree <size>\0. This is called the object header, and it works like the header in blob objects.
// After the header, the file contains several entries.
//
// Each entry follows the format <mode> <name>\0<sha>:
//
//	   <mode> shows the type and permissions of the file or directory.
//	   <name> is the name of the file or directory.
//	   \0 represents a null byte.
//	   <20_byte_sha> is the 20-byte SHA-1 hash of the file or directory.
//
// The <mode> Field
// The <mode> field shows the type and permissions for each entry. Some valid values include:
//
// 100644 - Regular file
// 100755 - Executable file
// 40000 - Directory (Tree object)
// Note that directory mode is 40000, not 040000.
// Although Git commands like git ls-tree show directory modes as 040000 for readability,
// the actual mode stored in the tree object is 40000.

func writeDir(dirName string, currPath string, buffer *bytes.Buffer) {
	dirEnts, err := os.ReadDir(dirName)

	if err != nil {
		fmt.Printf("Failed to read directory: %s\n", dirName)
		os.Exit(1)
	}

	for _, entry := range dirEnts {
		info, err := entry.Info()

		// Write a file/symlink
		mode := ""

		if entry.IsDir() {
			mode = "40000"
		} else if info.Mode()&os.ModeSymlink != 0 {
			// symlink
			mode = "120000"
		} else if info.Mode()&0111 != 0 {
			// executable
			mode = "100755"
		} else {
			// regular file
			mode = "100644"
		}

		if entry.IsDir() {
			if entry.Name() == ".git" {
				continue
			}
			
			var newBuffer bytes.Buffer
			writeDir(entry.Name(), filepath.Join(currPath, entry.Name()), &newBuffer)

			// Now commit this as a tree object
			treeHash := commitObject(".", &newBuffer, "tree")

			// <mode> <name>\0<20_byte_sha (not hex)>
			buffer.Write([]byte(mode))
			buffer.WriteByte(' ')
			buffer.Write([]byte(filepath.Join(currPath, entry.Name())))
			buffer.WriteByte(0)
			buffer.Write(treeHash)

			continue
		}

		if err != nil {
			fmt.Printf("Failed to get info for: %s\n", entry.Name())
			os.Exit(1)
		}

		// <mode> <name>\0<20_byte_sha (not hex)>
		buffer.Write([]byte(mode))
		buffer.WriteByte(' ')
		buffer.Write([]byte(entry.Name()))
		buffer.WriteByte(0)

		fullFilePath := filepath.Join(currPath, entry.Name())
		hashBytes := hashFile(fullFilePath, true, false)
		buffer.Write(hashBytes)
	}
}

// Create a git tree object recursively
func writeTree() {
	// We'll write the header later as we don't have size right now
	var buffer bytes.Buffer
	writeDir(".", "", &buffer)

	// Now we have the final buffer, write the tree object
	finalTreeHash := commitObject(".", &buffer, "tree")
	fmt.Printf("%x\n", finalTreeHash)
}
