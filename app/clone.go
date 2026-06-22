package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PackHeader struct {
	// PACK
	Signature [4]byte
	Version   uint32
	// Count of objects
	//
	// Could be commits + trees + blobs
	Count uint32
}

type ObjType uint8

const (
	ObjTypeCommit   ObjType = 1
	ObjTypeTree     ObjType = 2
	ObjTypeBlob     ObjType = 3
	ObjTypeTag      ObjType = 4
	ObjTypeOfsDelta ObjType = 6
	ObjTypeRefDelta ObjType = 7
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

// Just writes down the uncompressed object commit,blob etc to disk
func handleObject(cloneDir string, objType ObjType, uncompressedData []byte) {
	switch objType {
	case ObjTypeCommit:
		{
			commitObject(cloneDir, bytes.NewBuffer(uncompressedData), "commit")
			// fmt.Printf("Wrote commit object: %x\n", hash)
		}
	case ObjTypeTree:
		{
			commitObject(cloneDir, bytes.NewBuffer(uncompressedData), "tree")
			// fmt.Printf("Wrote tree object: %x\n", hash)
		}
	case ObjTypeBlob:
		{
			commitObject(cloneDir, bytes.NewBuffer(uncompressedData), "blob")
			// fmt.Printf("Wrote blob object: %x\n", hash)
		}
	case ObjTypeTag:
		{
			fmt.Println("ObjTypeTag")
		}
	case ObjTypeOfsDelta:
		{
			fmt.Println("ObjTypeOfsDelta")
		}
	case ObjTypeRefDelta:
		{
			fmt.Println("ObjTypeRefDelta")
		}

	default:
		{
			fmt.Printf("Unknown type: %d\n", objType)
			os.Exit(1)
		}
	}

}

func parseSizeAndObjType(buffer *bytes.Buffer) (int, ObjType) {
	// Object header format
	//
	// First byte:
	//
	// MSB        = continuation bit (C)
	// bits 4-6   = type (T)
	// bits 0-3   = size (low 4 bits) (S)
	//
	//  7 6 5 4 3 2 1 0
	// |C T T T S S S S|

	// cttt & 0111

	objectType := [1]byte{}
	binary.Read(buffer, binary.BigEndian, &objectType)

	continueSet := objectType[0]>>7 == 1
	objType := (objectType[0] >> 4) & 0b0111

	// 0000 1111 => MSB + 3 bits for type rest is size
	size := []byte{objectType[0] & 0b00001111}

	for continueSet {
		// Keep going through the bytes and keep getting the
		// 7 LSB to be able to finally concat and get size
		binary.Read(buffer, binary.BigEndian, &objectType)

		continueSet = (objectType[0] >> 7) == 1
		size = append(size, objectType[0]&0b01111111)
	}

	// Size Encoding
	//
	// From each byte, the seven least significant bits are used to form the resulting integer.
	// As long as the most significant bit is 1, this process continues; the byte with MSB 0 provides the last seven bits.
	// The seven-bit chunks are concatenated. Later values are more significant.

	totalSize := int(size[0])

	shift := uint(4)

	for i := 1; i < len(size); i++ {
		b := size[i]
		totalSize |= int(b) << shift
		shift += 7
	}

	return totalSize, ObjType(objType)
}

// delta objects have no obj type bit
func parseDeltaSize(buffer *bytes.Buffer) int {
	size := 0
	shift := uint(0)

	for {
		b, _ := buffer.ReadByte()
		size |= int(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}

	return size
}

type RefDelta struct {
	// The base sha
	baseShaHex string
	// The size we get from parsing object header
	totalSize int
	// Rest of the zlib uncompressed refdelta data
	data []byte
}

// Spec: https://git-scm.com/docs/pack-format#_deltified_representation
func parseRefDelta(cloneDir string, delta RefDelta) bool {
	if !gitObjectExists(cloneDir, delta.baseShaHex) {
		return false
	}

	buffer := bytes.NewBuffer(delta.data)

	// The delta data starts with the size of the base object and the size of the object to be reconstructed.
	// These sizes are encoded using the size encoding from above.

	// We don't care about the type here are they're bogus-amogus
	totalSizeBaseObj := parseDeltaSize(buffer)
	totalSizeReconObj := parseDeltaSize(buffer)

	// The remainder of the delta data is a sequence of instructions to reconstruct the object from the base object
	// There are two supported instructions so far:
	//
	// one for copy a byte range from the source object and
	// one for inserting new data embedded in the instruction itself.

	// Each instruction has variable length.
	// Instruction type is determined by the seventh bit (counting from 0, so basically the very first bit)
	// of the first octet

	// Instruction to copy from base object

	// below
	// s = bit for size
	// o = bit for offset
	// counted from right to left

	// +----------+---------+---------+---------+---------+-------+-------+-------+
	// | 1 sss oooo | offset1 | offset2 | offset3 | offset4 | size1 | size2 | size3 |
	// +----------+---------+---------+---------+---------+-------+-------+-------+

	// This is the instruction format to copy a byte range from the source object.
	// It encodes the offset to copy from and the number of bytes to copy.
	// Offset and size are in little-endian order.

	// The first seven bits in the first octet determines which of the next seven octets is present.
	// If bit zero is set, offset1 is present. If bit one is set offset2 is present and so on.

	// this only encodes ONE offset into base and the size to copy and NOT multiple offsets

	// Now to copy the data and create a new object
	baseObjFile := readGitObject(cloneDir, delta.baseShaHex)
	baseObjFileDecom := decompressGitObj(baseObjFile)

	if gitObjectSize(baseObjFileDecom) != totalSizeBaseObj {
		fmt.Printf(
			"base obj data has size %d, expected %d\n",
			gitObjectSize(baseObjFileDecom),
			totalSizeBaseObj,
		)
		os.Exit(1)
	}

	// The base sha might be blob,tree,commit
	// Get this first as we modify baseObjFileDecom this afterwards
	baseObjectType := gitObjType(baseObjFileDecom)

	// Extract only the data part of the base file
	// Skip <commit|tree|blob> <size>\0
	nullIdx := findFirstNull(baseObjFileDecom, 0)

	if nullIdx == -1 {
		fmt.Printf("Bad git object. Could not find null byte")
		os.Exit(1)
	}

	baseObjFileDecom = baseObjFileDecom[nullIdx+1:]

	reconstructed := []byte{}

	for buffer.Len() > 0 {
		firstByte, _ := buffer.ReadByte()

		instruction := (firstByte >> 7)

		switch instruction {
		// Copy from base object case
		case 1:
			{
				offsetIntoBase := 0
				sizeToCopy := 0

				// check which offsets are present
				if firstByte&0b1 != 0 {
					// Read the next byte
					b, _ := buffer.ReadByte()
					offsetIntoBase |= int(b)
				}

				if firstByte&0b10 != 0 {
					// Read the next byte
					b, _ := buffer.ReadByte()
					offsetIntoBase |= int(b) << 8
				}

				if firstByte&0b100 != 0 {
					// Read the next byte
					b, _ := buffer.ReadByte()
					offsetIntoBase |= int(b) << 16
				}

				if firstByte&0b1000 != 0 {
					// Read the next byte
					b, _ := buffer.ReadByte()
					offsetIntoBase |= int(b) << 24
				}

				// Now we get the size
				if firstByte&0b10000 != 0 {
					b, _ := buffer.ReadByte()
					sizeToCopy |= int(b)
				}
				if firstByte&0b100000 != 0 {
					b, _ := buffer.ReadByte()
					sizeToCopy |= int(b) << 8
				}
				if firstByte&0b1000000 != 0 {
					b, _ := buffer.ReadByte()
					sizeToCopy |= int(b) << 16
				}

				if sizeToCopy == 0 {
					// There is another exception: size zero is automatically converted to 0x10000.
					sizeToCopy = 0x10000
				}

				reconstructed = append(
					reconstructed,
					baseObjFileDecom[offsetIntoBase:offsetIntoBase+sizeToCopy]...)
			}

		// Add new data instruction
		case 0:
			{
				// Insert instruction: the remaining 7 bits contain the size of data to insert
				insertSize := int(firstByte & 0b01111111)
				if insertSize == 0 {
					fmt.Printf("Size for append instruction must be non-zero")
					os.Exit(1)
				}

				// Read the data to insert
				insertData := make([]byte, insertSize)
				buffer.Read(insertData)
				reconstructed = append(reconstructed, insertData...)
			}
		}

		// Stop if we've reached the expected size
		if len(reconstructed) >= totalSizeReconObj {
			break
		}
	}

	if len(reconstructed) != totalSizeReconObj {
		fmt.Printf(
			"Reconstructed data has size %d, expected %d\n",
			len(reconstructed),
			totalSizeReconObj,
		)
		os.Exit(1)
	}

	buf := bytes.NewBuffer(reconstructed)
	// The base sha might be blob,tree,commit
	fmt.Printf("baseObjectType: %s\n", baseObjectType)
	commitObject(cloneDir, buf, baseObjectType)

	fmt.Println("---------------------------")

	return true
}

func parseObjects(cloneDir string, buffer *bytes.Buffer, objNum uint32) []RefDelta {
	unprocessableRefDeltas := []RefDelta{}

	// Now parse objects
	// (undeltified representation)
	// n-byte type and length (3-bit type, (n-1)*7+4-bit length)
	// compressed data

	totalSize, objType := parseSizeAndObjType(buffer)

	switch objType {
	case ObjTypeCommit, ObjTypeTree, ObjTypeBlob, ObjTypeTag:
		{
			// Now we read exactly `totalSize` amount of uncompressed bytes
			reader, err := zlib.NewReader(buffer)

			if err != nil {
				fmt.Printf("Failed to create zlib reader: %+v\n", err)
				os.Exit(1)
			}

			uncompressedData := make([]byte, totalSize)
			n, err := io.ReadFull(reader, uncompressedData)

			if err != nil {
				fmt.Printf("Failed to read zlib reader: %+v\n", err)
				os.Exit(1)
			}

			if n != totalSize {
				fmt.Printf("n: %d, totalSize: %d\n", n, totalSize)
				os.Exit(1)
			}

			// If size is 0 then discard the empty zlib stream
			// zlib compressed empty data = 789c 0300 0000 0001
			// but since size is 0, we en up only reading the zlib header
			// i.e. 789c which bites us in the ass afterwards as we try to
			// parse 0300... as a PACK header
			io.Copy(io.Discard, reader)

			if err := reader.Close(); err != nil {
				fmt.Printf("Failed to close zlib reader")
				os.Exit(1)
			}

			handleObject(cloneDir, ObjType(objType), uncompressedData)
		}

	case ObjTypeOfsDelta:
		fmt.Println("ObjTypeOfsDelta")

	case ObjTypeRefDelta:
		{
			// Read the base sha
			//
			// This is the base object
			// The deltas on this base object are stored in the zlib compressed
			// data
			baseSha := make([]byte, 20)
			io.ReadFull(buffer, baseSha)
			baseShaHex := fmt.Sprintf("%x", baseSha)

			// Even if baseShaHex doesn't exist we still need to parse the zlib compressed data
			zr, err := zlib.NewReader(buffer)
			if err != nil {
				panic(err)
			}

			uncompressedData := make([]byte, totalSize)
			_, err = io.ReadFull(zr, uncompressedData)
			if err != nil {
				panic(err)
			}

			zr.Close()

			refDelta := RefDelta{
				baseShaHex: baseShaHex,
				totalSize:  totalSize,
				data:       uncompressedData,
			}

			if !parseRefDelta(cloneDir, refDelta) {
				// weren't able to parse it, base sha does not exist yet
				unprocessableRefDeltas = append(unprocessableRefDeltas, refDelta)
			} else {
				fmt.Printf("Parsed ref delta for base: %s\n", baseShaHex)
			}
		}

	default:
		{
			fmt.Printf(
				"parseObjects: Unknown type: %d parsing object number: %d\n",
				objType,
				objNum,
			)
			os.Exit(1)
		}
	}

	return unprocessableRefDeltas
}

func parsePackFile(cloneDir string, data []byte) {
	// Header - 0008NAK\n (skip it)
	if string(data[:8]) != "0008NAK\n" {
		fmt.Printf("Invalid NAK header. Got: %s\n", string(data[:8]))
		os.Exit(1)
	}

	buffer := bytes.NewBuffer(data[8:])

	packHeader := PackHeader{}
	binary.Read(buffer, binary.BigEndian, &packHeader)

	// Now expect PACK
	if packHeader.Signature != [4]byte{'P', 'A', 'C', 'K'} {
		fmt.Printf("Invalid PACK header. Got: %s\n", packHeader.Signature)
		os.Exit(1)
	}

	unprocessableRefDeltas := []RefDelta{}

	for i := range packHeader.Count {
		ret := parseObjects(cloneDir, buffer, i)
		unprocessableRefDeltas = append(unprocessableRefDeltas, ret...)
	}

	fmt.Printf(
		"Parsed most objects. RefDeltas that need processing: %d\n",
		len(unprocessableRefDeltas),
	)

	// process the unprocessableRefDeltas
	for len(unprocessableRefDeltas) > 0 {
		needsProcessing := []RefDelta{}

		for _, delta := range unprocessableRefDeltas {
			if !parseRefDelta(cloneDir, delta) {
				needsProcessing = append(needsProcessing, delta)
			}
		}

		fmt.Printf("Parsed %d RefDeltas\n", len(unprocessableRefDeltas)-len(needsProcessing))

		if len(needsProcessing) == len(unprocessableRefDeltas) {
			fmt.Printf(
				"Could not parse a single refDelta after one iteration: Before: (%d items), After: (%d items)\n",
				len(unprocessableRefDeltas),
				len(needsProcessing),
			)
			os.Exit(1)
		}

		unprocessableRefDeltas = needsProcessing
	}
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

	for line := range strings.SplitSeq(bodyString, "\n") {
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
		fmt.Println("Could not find HEAD")
		os.Exit(1)
	}

	var postBody bytes.Buffer

	want := fmt.Sprintf("want %s\n", headSha)
	// the PKT-Line length
	postBody.WriteString(fmt.Sprintf("%04x", len(want)+4))
	postBody.WriteString(want)
	postBody.WriteString("0000")
	postBody.WriteString("0009done\n")

	postReq, err := http.Post(
		fmt.Sprintf("%s/git-upload-pack", url),
		"application/x-git-upload-pack-request",
		&postBody,
	)

	if err != nil {
		fmt.Printf("Failed to send git-upload-pack request\n")
		os.Exit(1)
	}

	postResp, err := io.ReadAll(postReq.Body)

	if err != nil {
		fmt.Printf("Failed to read git-upload-pack response: %+v\n", err)
		os.Exit(1)
	}

	parsePackFile(cloneDir, postResp)

	// From the very first commit, get the tree then we create the directory
	// structure from the tree

	// Read the tree object from headsha
	file := readGitObject(cloneDir, headSha)
	decompressed := decompressGitObj(file)

	// first line in commit is commit <size>\0tree <>...
	nullIdx := findFirstNull(decompressed, 0)

	if nullIdx == -1 {
		fmt.Printf("Malformed commit: %s\n", headSha)
		os.Exit(1)
	}

	decompressedStr := string(decompressed[nullIdx:])

	tree := strings.Split(decompressedStr, "\n")[0]
	treeSha := strings.TrimSpace(tree[5:])

	treeToFs(cloneDir, treeSha)
}
