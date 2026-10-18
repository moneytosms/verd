# {{.Problem.ID}} {{.Problem.Name}}
# {{.Problem.URL}}
# {{.Handle}}, {{.Date}}
import os
import sys
from pprint import pformat

input = sys.stdin.readline
# debug(f"{x=}", y): stderr, only in verd runs (LOCAL set)
debug = (lambda *a: print(*(x if isinstance(x, str) else pformat(x) for x in a), file=sys.stderr)) if os.getenv("LOCAL") else (lambda *a: None)


def solve():
    {{cursor}}


for _ in range(int(input())):
    solve()
