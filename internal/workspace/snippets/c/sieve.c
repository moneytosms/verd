// sieve of Eratosthenes: returns a malloc'd array with is_prime[i] for 0 <= i <= n in O(n log log n)
#include <stdlib.h>
#include <string.h>
char *sieve(int n) {
    char *p = malloc(n + 1);
    memset(p, 1, n + 1);
    p[0] = 0;
    if (n >= 1) p[1] = 0;
    for (long long i = 2; i * i <= n; i++)
        if (p[i])
            for (long long j = i * i; j <= n; j += i) p[j] = 0;
    return p;
}
