# debug print to stderr: dbg(x, y) prints the values
import sys


def dbg(*args):
    print(*args, file=sys.stderr)
