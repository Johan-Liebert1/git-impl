package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func createFsTree(repoDir string, dirfd int, treeSha string) {
	var bytes bytes.Buffer
	lsTree(&bytes, repoDir, treeSha, false)

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
				fmt.Printf("Failed to create dirctory '%s': %+v\n", name, err)
				os.Exit(1)
			}

			dirfd, err := unix.Openat(dirfd, name, unix.O_DIRECTORY, 0)

			if err != nil {
				fmt.Printf("Failed to open dirctory '%s': %+v\n", name, err)
				os.Exit(1)
			}

			createFsTree(repoDir, dirfd, sha)

			continue
		}

		// Create the file
		obj := readGitObject(repoDir, sha)
		decompressedFile := decompressGitObj(obj)

		// skip the header
		nullIdx := findFirstNull(decompressedFile, 0)

		if nullIdx == -1 {
			fmt.Printf("Bad git object. Could not find null byte to get size")
			os.Exit(1)
		}

		decompressedFile = decompressedFile[nullIdx+1:]

		perms := 0

		if mode == "100644" {
			perms = 0o644
		} else {
			perms = 0o755
		}

		fd, err := unix.Openat(dirfd, name, unix.O_CREAT|unix.O_RDWR, uint32(perms))

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
func treeToFs(repoDir string, treeSha string) {
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		fmt.Printf("Failed to create dir '%s': %+v\n", repoDir, err)
		os.Exit(1)
	}

	dirfd, err := unix.Open(repoDir, unix.O_DIRECTORY, 0)

	if err != nil {
		fmt.Printf("Failed to open dir '%s': %+v\n", repoDir, err)
		os.Exit(1)
	}

	// Min things .git needs to be considered a repo

	// .git/
	// ├── HEAD = ref: refs/heads/master
	// ├── objects
	// └── refs

	if err := unix.Mkdirat(dirfd, ".git/refs", 0o755); err != nil {
		fmt.Printf("Failed to create refs dir: %+v\n", err)
		os.Exit(1)
	}

	headFd, err := unix.Openat(dirfd, ".git/HEAD", unix.O_CREAT|unix.O_RDWR, 0o644)
	if err != nil {
		fmt.Printf("Failed to create HEAD: %+v\n", err)
		os.Exit(1)
	}

	if _, err := unix.Write(headFd, []byte("ref: refs/heads/master\n")); err != nil {
		fmt.Printf("Failed to write to HEAD: %+v\n", err)
		os.Exit(1)
	}

	createFsTree(repoDir, dirfd, treeSha)
}
