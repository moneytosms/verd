// read n vertices and m weighted undirected edges (1-indexed) into g[u] = {(v, w), ...}
int n, m;
cin >> n >> m;
vector<vector<pair<int, long long>>> g(n);
for (int i = 0; i < m; i++) {
    int u, v;
    long long w;
    cin >> u >> v >> w;
    u--, v--;
    g[u].push_back({v, w});
    g[v].push_back({u, w});
}
