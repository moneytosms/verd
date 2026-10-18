// all-pairs shortest paths in O(n^3): d[i][j] = edge weight or INF, d[i][i] = 0 (n <= FW_N)
#define FW_N 505
#define INF 1000000000000000000LL
void floyd_warshall(int n, long long d[][FW_N]) {
    for (int k = 0; k < n; k++)
        for (int i = 0; i < n; i++)
            for (int j = 0; j < n; j++)
                if (d[i][k] < INF && d[k][j] < INF && d[i][k] + d[k][j] < d[i][j]) d[i][j] = d[i][k] + d[k][j];
}
