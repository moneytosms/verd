// deterministic Miller-Rabin primality test for any 64-bit n
bool is_prime(unsigned long long n) {
    if (n < 2) return false;
    for (unsigned long long p : {2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37}) {
        if (n % p == 0) return n == p;
    }
    auto mulmod = [](unsigned long long a, unsigned long long b, unsigned long long m) { return (unsigned long long)((__uint128_t)a * b % m); };
    auto powmod = [&](unsigned long long b, unsigned long long e, unsigned long long m) {
        unsigned long long r = 1;
        for (b %= m; e; e >>= 1, b = mulmod(b, b, m))
            if (e & 1) r = mulmod(r, b, m);
        return r;
    };
    unsigned long long d = n - 1;
    int s = 0;
    while (d % 2 == 0) d /= 2, s++;
    for (unsigned long long a : {2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37}) {
        unsigned long long x = powmod(a, d, n);
        if (x == 1 || x == n - 1) continue;
        bool composite = true;
        for (int i = 1; i < s && composite; i++) {
            x = mulmod(x, x, n);
            if (x == n - 1) composite = false;
        }
        if (composite) return false;
    }
    return true;
}
