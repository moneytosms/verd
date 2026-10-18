// {{.Problem.ID}} {{.Problem.Name}}
// {{.Problem.URL}}
// {{.Handle}}, {{.Date}}
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef LOCAL
#define debug(fmt, x) fprintf(stderr, #x " = " fmt "\n", x)
#else
#define debug(fmt, x)
#endif

typedef long long ll;
typedef unsigned long long ull;
#define fors(i, n) for (int i = 0; i < (n); i++)
#define min(a, b) ((a) < (b) ? (a) : (b))
#define max(a, b) ((a) > (b) ? (a) : (b))

int read_int(void) { int x; scanf("%d", &x); return x; }
ll read_ll(void) { ll x; scanf("%lld", &x); return x; }
void YES(int b) { puts(b ? "YES" : "NO"); }

void solve(void) {
    {{cursor}}
}

int main(void) {
    int T = 1;
    scanf("%d", &T);
    while (T--) solve();
    return 0;
}
