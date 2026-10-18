// topological order of a directed graph (Kahn) over the graphinput arrays; returns how many vertices were ordered (less than n means a cycle)
int topo_sort(int n, int *order) {
    static int indeg[MAXN];
    int h = 0, t = 0;
    for (int u = 0; u < n; u++) indeg[u] = 0;
    for (int u = 0; u < n; u++)
        for (int e = head[u]; e >= 0; e = nxt[e]) indeg[to[e]]++;
    for (int u = 0; u < n; u++)
        if (!indeg[u]) order[t++] = u;
    while (h < t) {
        int u = order[h++];
        for (int e = head[u]; e >= 0; e = nxt[e])
            if (!--indeg[to[e]]) order[t++] = to[e];
    }
    return t;
}
