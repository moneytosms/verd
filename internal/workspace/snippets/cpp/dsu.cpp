// disjoint set union: find with path compression, unite by size (returns false if already joined)
struct DSU {
    vector<int> p, siz;
    DSU(int n) : p(n), siz(n, 1) { iota(p.begin(), p.end(), 0); }
    int find(int x) { return p[x] == x ? x : p[x] = find(p[x]); }
    bool unite(int a, int b) {
        a = find(a), b = find(b);
        if (a == b) return false;
        if (siz[a] < siz[b]) swap(a, b);
        p[b] = a;
        siz[a] += siz[b];
        return true;
    }
};
