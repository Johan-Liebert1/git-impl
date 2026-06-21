
import sys, zlib

name = sys.argv[1]

sys.stdout.buffer.write(
    zlib.compress(open(f"{name}", "rb").read())
)
