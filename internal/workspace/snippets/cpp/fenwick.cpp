// Fenwick tree (BIT), 0-indexed: add(i, v) is a point update, sum(l, r) is the sum of [l, r); O(log n)
struct Fenwick {
    int n;
    vector<long long> t;
    Fenwick(int n) : n(n), t(n + 1) {}
    void add(int i, long long v) {
        for (i++; i <= n; i += i & -i) t[i] += v;
    }
    long long sum(int i) {
        long long s = 0;
        for (; i > 0; i -= i & -i) s += t[i];
        return s;
    }
    long long sum(int l, int r) { return sum(r) - sum(l); }
};
