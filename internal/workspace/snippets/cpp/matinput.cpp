// read an n x m matrix of numbers
int n, m;
cin >> n >> m;
vector<vector<long long>> g(n, vector<long long>(m));
for (auto &row : g)
    for (auto &x : row) cin >> x;
