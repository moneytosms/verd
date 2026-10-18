// disjoint set union: dsu_init(n), dsu_find with path compression, dsu_unite by size (returns 0 if already joined)
#define DSU_N 200005
int dsu_p[DSU_N], dsu_sz[DSU_N];
void dsu_init(int n) {
    for (int i = 0; i < n; i++) dsu_p[i] = i, dsu_sz[i] = 1;
}
int dsu_find(int x) {
    while (dsu_p[x] != x) x = dsu_p[x] = dsu_p[dsu_p[x]];
    return x;
}
int dsu_unite(int a, int b) {
    a = dsu_find(a), b = dsu_find(b);
    if (a == b) return 0;
    if (dsu_sz[a] < dsu_sz[b]) {
        int t = a;
        a = b;
        b = t;
    }
    dsu_p[b] = a;
    dsu_sz[a] += dsu_sz[b];
    return 1;
}
