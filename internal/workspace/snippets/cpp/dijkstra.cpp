// Dijkstra from s over g[u] = {(v, w), ...} with non-negative weights, O((n + m) log n); LLONG_MAX = unreachable
vector<long long> dijkstra(const vector<vector<pair<int, long long>>> &g, int s) {
    vector<long long> dist(g.size(), LLONG_MAX);
    priority_queue<pair<long long, int>, vector<pair<long long, int>>, greater<>> pq;
    dist[s] = 0;
    pq.push({0, s});
    while (!pq.empty()) {
        auto [d, u] = pq.top();
        pq.pop();
        if (d > dist[u]) continue;
        for (auto [v, w] : g[u])
            if (d + w < dist[v]) {
                dist[v] = d + w;
                pq.push({dist[v], v});
            }
    }
    return dist;
}
