// Dijkstra from s over the graphinput arrays with non-negative weights, O((n + m) log n); LLONG_MAX = unreachable
#include <limits.h>
typedef struct {
    long long d;
    int v;
} HeapItem;
HeapItem heap[MAXM + 1];
int hn;
void hpush(long long d, int v) {
    int i = hn++;
    while (i > 0) {
        int p = (i - 1) / 2;
        if (heap[p].d <= d) break;
        heap[i] = heap[p];
        i = p;
    }
    heap[i].d = d;
    heap[i].v = v;
}
HeapItem hpop(void) {
    HeapItem top = heap[0], last = heap[--hn];
    int i = 0;
    while (2 * i + 1 < hn) {
        int c = 2 * i + 1;
        if (c + 1 < hn && heap[c + 1].d < heap[c].d) c++;
        if (heap[c].d >= last.d) break;
        heap[i] = heap[c];
        i = c;
    }
    heap[i] = last;
    return top;
}
long long dij_dist[MAXN];
void dijkstra(int n, int s) {
    for (int i = 0; i < n; i++) dij_dist[i] = LLONG_MAX;
    hn = 0;
    dij_dist[s] = 0;
    hpush(0, s);
    while (hn > 0) {
        HeapItem it = hpop();
        if (it.d > dij_dist[it.v]) continue;
        for (int e = head[it.v]; e >= 0; e = nxt[e]) {
            int v = to[e];
            if (it.d + wt[e] < dij_dist[v]) {
                dij_dist[v] = it.d + wt[e];
                hpush(dij_dist[v], v);
            }
        }
    }
}
