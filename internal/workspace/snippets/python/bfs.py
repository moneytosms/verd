# BFS from s over adjacency lists g: dist[v] is the number of edges from s, -1 if unreachable
from collections import deque


def bfs(g, s):
    dist = [-1] * len(g)
    dist[s] = 0
    q = deque([s])
    while q:
        u = q.popleft()
        for v in g[u]:
            if dist[v] < 0:
                dist[v] = dist[u] + 1
                q.append(v)
    return dist
