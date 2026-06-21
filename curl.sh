#!/bin/bash

GIT_URL="https://github.com/Johan-Liebert1/git-impl"

# Very first thing to do
curl "${GIT_URL}/info/refs?service=git-upload-pack" --output - -i

# The above will return something like

# 001e# service=git-upload-pack
# 0000015b91f1ec7f403afb4deace4ed33b416a0deeeb1ee4 HEADmulti_ack thin-pack side-band side-band-64k ofs-delta shallow deepen-since deepen-not deepen-relative no-progress include-tag multi_ack_detailed allow-tip-sha1-in-want allow-reachable-sha1-in-want no-done symref=HEAD:refs/heads/master filter object-format=sha1 agent=git/github-651a2e299cdb-Linux
# 003f91f1ec7f403afb4deace4ed33b416a0deeeb1ee4 refs/heads/master
# 0000%                                                                                                                                                                                          


# A pkt-line is Git's framing protocol. Every message is:
#
# <4 hex digits><payload>
#
# The 4 hex digits represent the total length, including the 4 length bytes themselves.
#
# Example

# Payload:
#
# hello\n
#
# Payload size:
#
# hello = 5 bytes
# \n    = 1 byte
# -------------
# total payload = 6 bytes
#
# Pkt-line size: 4 (length field) + 6 (payload) = 10 bytes
# 10 decimal = 000a hex
#
# So the pkt-line is: 000ahello\n
#
#
# Flush packet

# A special pkt-line:
#
# 0000
#
# means: End of this section. No payload follows.
#
# Example:
#
# 0032want <sha>\n
# 0000
#
# means:
# One pkt-line containing a want. End of wants.

# Generating request.bin

SHA=91f1ec7f403afb4deace4ed33b416a0deeeb1ee4 # From the first response
{
    printf "0032want %s\n" "$SHA"
    printf "0000"
    printf "0009done\n"
} > request.bin

curl -X POST "${GIT_URL}/git-upload-pack" --output - -i \
    -H 'Content-Type: application/x-git-upload-pack-request' \
    --data-binary '@request.bin'
