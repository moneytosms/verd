// BFS from s over adjacency lists g: dist[v] is the number of edges from s, -1 if unreachable
vector<int> bfs(const vector<vector<int>> &g, int s) {
    vector<int> dist(g.size(), -1);
    queue<int> q;
    dist[s] = 0;
    q.push(s);
    while (!q.empty()) {
        int u = q.front();
        q.pop();
        for (int v : g[u])
            if (dist[v] < 0) {
                dist[v] = dist[u] + 1;
                q.push(v);
            }
    }
    return dist;
}
