// BFS from s over the graphinput arrays: dist[v] is the number of edges from s, -1 if unreachable
int dist[MAXN], bq[MAXN];
void bfs(int n, int s) {
    for (int i = 0; i < n; i++) dist[i] = -1;
    int h = 0, t = 0;
    dist[s] = 0;
    bq[t++] = s;
    while (h < t) {
        int u = bq[h++];
        for (int e = head[u]; e >= 0; e = nxt[e]) {
            int v = to[e];
            if (dist[v] < 0) {
                dist[v] = dist[u] + 1;
                bq[t++] = v;
            }
        }
    }
}
