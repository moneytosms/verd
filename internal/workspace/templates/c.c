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
#define fors(i, n) for (int i = 0; i < (n); i++)

void solve(void) {
    {{cursor}}
}

int main(void) {
    int T = 1;
    scanf("%d", &T);
    while (T--) solve();
    return 0;
}
