# {{.Problem.ID}} {{.Problem.Name}}
# {{.Problem.URL}}
# {{.Handle}}, {{.Date}}
import os
import sys
from pprint import pformat

input = sys.stdin.readline
# debug(f"{x=}", y): stderr, only in verd runs (LOCAL set)
debug = (lambda *a: print(*(x if isinstance(x, str) else pformat(x) for x in a), file=sys.stderr)) if os.getenv("LOCAL") else (lambda *a: None)

sys.setrecursionlimit(1 << 25)
INF = float("inf")
MOD = 10**9 + 7


def inp():
    return int(input())


def ints():
    return list(map(int, input().split()))


def YES(b=True):
    print("YES" if b else "NO")


def solve():
    {{cursor}}


for _ in range(inp()):
    solve()
