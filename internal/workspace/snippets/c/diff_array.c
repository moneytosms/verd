// difference array: range_add adds v to a[l..r) in O(1); diff_apply prefix-sums d (n entries) into the final values
void range_add(long long *d, int l, int r, long long v) {
    d[l] += v;
    d[r] -= v;
}
void diff_apply(long long *d, int n) {
    for (int i = 1; i < n; i++) d[i] += d[i - 1];
}
