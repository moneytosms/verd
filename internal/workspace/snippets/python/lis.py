# length of the longest strictly increasing subsequence in O(n log n)
from bisect import bisect_left


def lis(a):
    t = []
    for x in a:
        i = bisect_left(t, x)
        if i == len(t):
            t.append(x)
        else:
            t[i] = x
    return len(t)
