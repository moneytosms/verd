// read an n x m grid of characters, one string per row
int n, m;
cin >> n >> m;
vector<string> g(n);
for (auto &row : g) cin >> row;
