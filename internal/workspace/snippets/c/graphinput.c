// adjacency list in arrays: init_graph(n), add_edge(u, v, w) per directed edge, read_graph(n, m, weighted) for m undirected edges (1-indexed input). Paste before bfs, dfs, dijkstra and topo_sort.
#include <stdio.h>
#include <string.h>
#define MAXN 200005
#define MAXM 400010
int head[MAXN], nxt[MAXM], to[MAXM], ec;
long long wt[MAXM];
void init_graph(int n) {
    memset(head, -1, sizeof(int) * n);
    ec = 0;
}
void add_edge(int u, int v, long long w) {
    to[ec] = v;
    wt[ec] = w;
    nxt[ec] = head[u];
    head[u] = ec++;
}
void read_graph(int n, int m, int weighted) {
    init_graph(n);
    for (int i = 0; i < m; i++) {
        int u, v;
        long long w = 1;
        scanf("%d %d", &u, &v);
        if (weighted) scanf("%lld", &w);
        u--, v--;
        add_edge(u, v, w);
        add_edge(v, u, w);
    }
}
