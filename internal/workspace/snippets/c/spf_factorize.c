// smallest-prime-factor sieve: build_spf(n) once, then factorize(x, out) stores the prime factors of x <= n and returns how many
#include <stdlib.h>
int *spf;
void build_spf(int n) {
    spf = calloc(n + 1, sizeof *spf);
    for (int i = 2; i <= n; i++)
        if (!spf[i])
            for (int j = i; j <= n; j += i)
                if (!spf[j]) spf[j] = i;
}
int factorize(int x, int *out) {
    int k = 0;
    for (; x > 1; x /= spf[x]) out[k++] = spf[x];
    return k;
}
