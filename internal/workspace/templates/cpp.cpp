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
using ull = unsigned long long;
using pii = pair<int, int>;
using vi = vector<int>;
#define all(x) (x).begin(), (x).end()
#define sz(x) int((x).size())
#define pb push_back
#define F first
#define S second
#define fors(i, n) for (int i = 0; i < (n); i++)

template <class T> istream &operator>>(istream &is, vector<T> &v) { for (auto &x : v) is >> x; return is; }
template <class T> ostream &operator<<(ostream &os, const vector<T> &v) { for (size_t i = 0; i < v.size(); i++) os << (i ? " " : "") << v[i]; return os; }
void YES(bool b = true) { cout << (b ? "YES" : "NO") << '\n'; }
void NO(bool b = true) { YES(!b); }

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
