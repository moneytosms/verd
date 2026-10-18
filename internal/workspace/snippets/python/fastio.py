# fast input: replace input() with a buffered reader (about 3x faster on large inputs)
import sys

input = sys.stdin.readline
