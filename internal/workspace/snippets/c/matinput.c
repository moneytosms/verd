// read an n x m matrix of numbers (set the array size to the problem's limits)
int n, m;
static long long g[1005][1005];
scanf("%d %d", &n, &m);
for (int i = 0; i < n; i++)
    for (int j = 0; j < m; j++) scanf("%lld", &g[i][j]);
