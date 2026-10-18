// sparse table for range minimum on a static array: O(n log n) build, O(1) query of [l, r)
struct SparseTable {
    vector<vector<int>> t;
    SparseTable(const vector<int> &a) {
        int n = a.size(), k = 32 - __builtin_clz(n);
        t.assign(k, a);
        for (int j = 1; j < k; j++)
            for (int i = 0; i + (1 << j) <= n; i++) t[j][i] = min(t[j - 1][i], t[j - 1][i + (1 << (j - 1))]);
    }
    int query(int l, int r) {
        int j = 31 - __builtin_clz(r - l);
        return min(t[j][l], t[j][r - (1 << j)]);
    }
};
