// binary search on the answer: smallest x in [lo, hi] with ok(x) true (ok must go false...true); hi + 1 if none
long long first_true(long long lo, long long hi, int (*ok)(long long)) {
    long long l = lo, r = hi + 1;
    while (l < r) {
        long long mid = l + (r - l) / 2;
        if (ok(mid)) r = mid;
        else l = mid + 1;
    }
    return l;
}
