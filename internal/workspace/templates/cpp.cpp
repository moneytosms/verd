// {{.Problem.ID}} {{.Problem.Name}}
// {{.Problem.URL}}
// {{.Handle}}, {{.Date}}
#include <bits/stdc++.h>
using namespace std;

#ifdef LOCAL
#include "debug.h"
#else
#define debug(...) 42
#endif

using ll = long long;
#define all(x) (x).begin(), (x).end()
#define fors(i, n) for (int i = 0; i < (n); i++)

void solve() {
    {{cursor}}
}

int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int T = 1;
    cin >> T;
    while (T--) solve();
    return 0;
}
