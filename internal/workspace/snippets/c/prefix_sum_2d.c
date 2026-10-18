// 2D prefix sums of g (n x m): sum of rows [r1, r2) x cols [c1, c2) = p[r2][c2] - p[r1][c2] - p[r2][c1] + p[r1][c1]
static long long p[1006][1006];
for (int i = 0; i < n; i++)
    for (int j = 0; j < m; j++)
        p[i + 1][j + 1] = g[i][j] + p[i][j + 1] + p[i + 1][j] - p[i][j];
