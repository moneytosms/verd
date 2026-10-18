# all-pairs shortest paths in O(n^3): d[i][j] = edge weight or INF, d[i][i] = 0
INF = float("inf")


def floyd_warshall(d):
    n = len(d)
    for k in range(n):
        dk = d[k]
        for i in range(n):
            dik = d[i][k]
            if dik == INF:
                continue
            di = d[i]
            for j in range(n):
                if dik + dk[j] < di[j]:
                    di[j] = dik + dk[j]
