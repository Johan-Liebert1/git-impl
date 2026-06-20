package main

import (
	"bytes"
	"fmt"
	"os"
	"time"
)

// ./your_program.sh implCmdCommitTree-tree <tree_sha> -p <commit_sha> -m <message>
func implCmdCommitTree() {
	treeSha := os.Args[2]
	parentSha := os.Args[4]
	message := os.Args[6]

	author := "JohanLiebert"
	email := "johanliebert@511kinderheim.com"

	/*
		tree_sha -> string repr and not hash in bytes
		parent_sha -> string repr and not hash in bytes

		commit <size>\0tree <tree_sha>
		parent <parent_sha>
		author <name> <<email>> <timestamp> <timezone>
		committer <name> <<email>> <timestamp> <timezone>

		<commit message>
	*/

	authorStr := fmt.Sprintf(
		"%s %s %d %s",
		author,
		email,
		time.Now().Unix(),
		time.Now().Format("-0700"),
	)

	var buffer bytes.Buffer
	buffer.WriteString("tree")
	buffer.WriteByte(' ')
	buffer.WriteString(treeSha)
	buffer.WriteByte('\n')

	buffer.WriteString("parent")
	buffer.WriteByte(' ')
	buffer.WriteString(parentSha)
	buffer.WriteByte('\n')

	buffer.WriteString(fmt.Sprintf("author %s\n", authorStr))
	buffer.WriteString(fmt.Sprintf("committer %s\n", authorStr))

	// blank line between metadata and commit msg
	buffer.WriteByte('\n')
	buffer.WriteString(message)
	// new line after the commit message
	buffer.WriteByte('\n')

	hash := commitObject(&buffer, "commit")

	fmt.Printf("%x\n", hash)
}
