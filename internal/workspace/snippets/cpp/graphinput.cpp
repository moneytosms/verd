// read n vertices and m undirected edges (1-indexed in the input) into adjacency lists g
int n, m;
cin >> n >> m;
vector<vector<int>> g(n);
for (int i = 0; i < m; i++) {
    int u, v;
    cin >> u >> v;
    u--, v--;
    g[u].push_back(v);
    g[v].push_back(u);
}
