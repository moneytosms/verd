// topological order of a directed graph (Kahn); fewer than n vertices returned means a cycle
vector<int> topo_sort(const vector<vector<int>> &g) {
    int n = g.size();
    vector<int> indeg(n), order;
    for (int u = 0; u < n; u++)
        for (int v : g[u]) indeg[v]++;
    queue<int> q;
    for (int u = 0; u < n; u++)
        if (!indeg[u]) q.push(u);
    while (!q.empty()) {
        int u = q.front();
        q.pop();
        order.push_back(u);
        for (int v : g[u])
            if (!--indeg[v]) q.push(v);
    }
    return order;
}
