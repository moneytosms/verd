// iterative segment tree for sums: set(i, v) is a point update, query(l, r) the sum of [l, r); swap + for min/max
struct SegTree {
    int n;
    vector<long long> t;
    SegTree(int n) : n(n), t(2 * n) {}
    void set(int i, long long v) {
        for (t[i += n] = v; i > 1; i >>= 1) t[i >> 1] = t[i] + t[i ^ 1];
    }
    long long query(int l, int r) {
        long long s = 0;
        for (l += n, r += n; l < r; l >>= 1, r >>= 1) {
            if (l & 1) s += t[l++];
            if (r & 1) s += t[--r];
        }
        return s;
    }
};
