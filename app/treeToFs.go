package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func createFsTree(dirfd int, treeSha string) {
	var bytes bytes.Buffer
	lsTree(&bytes, treeSha, false)

	for line := range strings.SplitSeq(bytes.String(), "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		parts := strings.Split(line, " ")

		if len(parts) != 3 {
			fmt.Printf("Bad line in tree ouput '%s'\n", line)
			os.Exit(1)
		}

		mode, sha, name := parts[0], parts[1], parts[2]

		if mode == "40000" {
			// Create a directory and open tree with the sha
			err := unix.Mkdirat(dirfd, name, 0o755)

			if err != nil {
				fmt.Printf("Failed to create dirctory '%s'\n", name)
				os.Exit(1)
			}

			dirfd, err := unix.Openat(dirfd, name, unix.O_DIRECTORY, 0)

			if err != nil {
				fmt.Printf("Failed to open dirctory '%s'\n", name)
				os.Exit(1)
			}

			createFsTree(dirfd, sha)

			continue
		}

		// Create the file
		obj := readGitObject(sha)
		decompressedFile := decompressGitObj(obj)

		fd, err := unix.Openat(dirfd, name, unix.O_CREAT|unix.O_RDWR, 0)

		if err != nil {
			fmt.Printf("Failed to create file: %s. Err: %+v\n", name, err)
			os.Exit(1)
		}

		for len(decompressedFile) > 0 {
			n, err := unix.Write(fd, decompressedFile)

			if err != nil {
				fmt.Printf("Failed to write to file file: %s. Err: %+v\n", name, err)
				os.Exit(1)
			}

			decompressedFile = decompressedFile[n:]
		}
	}
}

// Accepts a tree object sha then creates a filesystem
// after parsing it
func treeToFs(workingDir string, treeSha string) {
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		fmt.Printf("Failed to create dir '%s': %+v\n", workingDir, err)
		os.Exit(1)
	}

	dirfd, err := unix.Open(workingDir, unix.O_DIRECTORY, 0)

	if err != nil {
		fmt.Printf("Failed to open dir '%s': %+v\n", workingDir, err)
		os.Exit(1)
	}

	createFsTree(dirfd, treeSha)
}
