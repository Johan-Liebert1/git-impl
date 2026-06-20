import sys, zlib

name = sys.argv[1]

sys.stdout.buffer.write(
    zlib.decompress(open(f".git/objects/{name[:2]}/{name[2:]}", "rb").read())
)
