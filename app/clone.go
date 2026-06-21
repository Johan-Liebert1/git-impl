package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// <4 hex digits><payload>
// The 4 hex digits represent the total length, including the 4 length bytes themselves.
func validatePktLine(line string) (bool, int) {
	if len(line) < 4 {
		return false, -1
	}

	length, err := strconv.ParseInt(line[:4], 16, 64)

	if err != nil {
		fmt.Printf("Failed to parse PKT-Line length. Invalid hex: '%s'", line[:4])
		return false, -1
	}

	// +1 for the new line since we split that
	// a proper parser would be better but ehhhh...
	return int(length) == len(line)+1, int(length)
}

func clone() {
	url := os.Args[2]
	cloneDir := os.Args[3]

	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		fmt.Printf("Failed to create dir: %s: %+v", cloneDir, err)
		os.Exit(1)
	}

	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s/info/refs?service=git-upload-pack", url),
		nil,
	)

	if err != nil {
		fmt.Printf("Failed to create request: %+v", err)
		os.Exit(1)
	}

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		fmt.Printf("Failed to send request: %+v", err)
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf("Failed to read body: %+v", err)
		os.Exit(1)
	}

	/*
		body should be something like

		001e# service=git-upload-pack
		0000015b91f1ec7f403afb4deace4ed33b416a0deeeb1ee4 HEADmulti_ack thin-pack side-band side-band-64k ofs-delta shallow deepen-since deepen-not deepen-relative no-progress include-tag multi_ack_detailed allow-tip-sha1-in-want allow-reachable-sha1-in-want no-done symref=HEAD:refs/heads/master filter object-format=sha1 agent=git/github-651a2e299cdb-Linux
		003f91f1ec7f403afb4deace4ed33b416a0deeeb1ee4 refs/heads/master
		0000

		1. Clients MUST validate the first five bytes of the response entity matches the regex ^[0-9a-f]{4}#. If this test fails, clients MUST NOT continue.
	*/

	reg := regexp.MustCompile("^[0-9a-f]{4}#")

	if !reg.Match(body) {
		fmt.Printf("git-upload-pack response is invalid: %+v", err)
		os.Exit(1)
	}

	// each line is a PKT-Line

	// A pkt-line is Git's framing protocol. Every message is:
	//
	// <4 hex digits><payload>
	//
	// The 4 hex digits represent the total length, including the 4 length bytes themselves.

	bodyString := string(body)
	headSha := ""

	for _, line := range strings.Split(bodyString, "\n") {
		if line[:4] == "0000" {
			// flush packet signalling separation between logical sections
			// ignore
			line = line[4:]
		}

		// very very bad
		// a proper parser is definitely needed here
		if len(line) == 0 {
			break
		}

		isValid, _ := validatePktLine(line)

		if !isValid {
			fmt.Printf("Invalid PKT-Line: %s\n", line)
			os.Exit(1)
		}

		// get the actual content
		// length is INCLUDING new-line and the first 4 payload bytes
		content := line[4:]

		if !strings.Contains(content, "HEAD") {
			// only care about the head. skip the rest of the branches
			continue
		}

		// Get the HEAD sha
		headSha = content[:40]
		break
	}

	if len(headSha) == 0 {
		fmt.Println("Could not find HEAD");
		os.Exit(1)
	}

	var postBody bytes.Buffer

	want := fmt.Sprintf("want %s\n", headSha)
	// the PKT-Line length
	postBody.WriteString(fmt.Sprintf("%04x", len(want) + 4))
	postBody.WriteString(want)
	postBody.WriteString("0000")
	postBody.WriteString("0009done\n")

	postReq, err := http.Post(fmt.Sprintf("%s/git-upload-pack", url), "application/x-git-upload-pack-request", &postBody)

	if err != nil {
		fmt.Printf("Failed to send git-upload-pack request\n")
		os.Exit(1)
	}

	postResp, err := io.ReadAll(postReq.Body)

	if err != nil {
		fmt.Printf("Failed to read git-upload-pack response: %+v\n", err)
		os.Exit(1)
	}

	f, _ := os.Create("resp")
	f.Write(postResp)
}
