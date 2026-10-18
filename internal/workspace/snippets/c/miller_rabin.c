// deterministic Miller-Rabin primality test for any 64-bit n (uses gcc's __int128)
typedef unsigned long long u64;
static u64 mulmod(u64 a, u64 b, u64 m) { return (u64)((unsigned __int128)a * b % m); }
static u64 powmod(u64 b, u64 e, u64 m) {
    u64 r = 1;
    for (b %= m; e; e >>= 1, b = mulmod(b, b, m))
        if (e & 1) r = mulmod(r, b, m);
    return r;
}
int is_prime(u64 n) {
    static const u64 bases[] = {2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37};
    if (n < 2) return 0;
    for (int i = 0; i < 12; i++)
        if (n % bases[i] == 0) return n == bases[i];
    u64 d = n - 1;
    int s = 0;
    while (d % 2 == 0) d /= 2, s++;
    for (int i = 0; i < 12; i++) {
        u64 x = powmod(bases[i], d, n);
        if (x == 1 || x == n - 1) continue;
        int composite = 1;
        for (int j = 1; j < s && composite; j++) {
            x = mulmod(x, x, n);
            if (x == n - 1) composite = 0;
        }
        if (composite) return 0;
    }
    return 1;
}
