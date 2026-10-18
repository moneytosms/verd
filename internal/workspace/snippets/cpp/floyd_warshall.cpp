// all-pairs shortest paths in O(n^3): d[i][j] = edge weight or INF, d[i][i] = 0
const long long INF = 1e18;
void floyd_warshall(vector<vector<long long>> &d) {
    int n = d.size();
    for (int k = 0; k < n; k++)
        for (int i = 0; i < n; i++)
            for (int j = 0; j < n; j++)
                if (d[i][k] < INF && d[k][j] < INF) d[i][j] = min(d[i][j], d[i][k] + d[k][j]);
}
