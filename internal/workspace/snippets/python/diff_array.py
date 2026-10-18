# difference array: range_add adds v to a[l:r] in O(1); diff_apply prefix-sums d into the final values
from itertools import accumulate


def range_add(d, l, r, v):
    d[l] += v
    d[r] -= v


def diff_apply(d):
    return list(accumulate(d))
