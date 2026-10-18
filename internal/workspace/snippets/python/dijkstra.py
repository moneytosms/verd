# Dijkstra from s over g[u] = [(v, w), ...] with non-negative weights, O((n + m) log n); inf = unreachable
import heapq


def dijkstra(g, s):
    dist = [float("inf")] * len(g)
    dist[s] = 0
    pq = [(0, s)]
    while pq:
        d, u = heapq.heappop(pq)
        if d > dist[u]:
            continue
        for v, w in g[u]:
            if d + w < dist[v]:
                dist[v] = d + w
                heapq.heappush(pq, (dist[v], v))
    return dist
