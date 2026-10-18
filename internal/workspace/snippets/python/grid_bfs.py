# BFS on a grid of '.' (free) and '#' (wall) from (sr, sc): steps to each cell, -1 if unreachable
from collections import deque


def grid_bfs(g, sr, sc):
    n, m = len(g), len(g[0])
    dist = [[-1] * m for _ in range(n)]
    dist[sr][sc] = 0
    q = deque([(sr, sc)])
    while q:
        r, c = q.popleft()
        for nr, nc in ((r - 1, c), (r + 1, c), (r, c - 1), (r, c + 1)):
            if 0 <= nr < n and 0 <= nc < m and g[nr][nc] != "#" and dist[nr][nc] < 0:
                dist[nr][nc] = dist[r][c] + 1
                q.append((nr, nc))
    return dist
