# binary search on the answer: smallest x in [lo, hi] with ok(x) true (ok must go False...True); hi + 1 if none
def first_true(lo, hi, ok):
    l, r = lo, hi + 1
    while l < r:
        mid = (l + r) // 2
        if ok(mid):
            r = mid
        else:
            l = mid + 1
    return l
