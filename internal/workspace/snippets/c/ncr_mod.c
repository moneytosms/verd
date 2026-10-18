// n choose r modulo a prime: build_fact(n) in O(n), then nCr(n, r) in O(1); fact and inv_fact need n + 1 entries
#define MOD 1000000007LL
#define MAXF 1000006
long long fact[MAXF], inv_fact[MAXF];
long long power(long long b, long long e) {
    long long r = 1;
    for (b %= MOD; e > 0; e >>= 1, b = b * b % MOD)
        if (e & 1) r = r * b % MOD;
    return r;
}
void build_fact(int n) {
    fact[0] = 1;
    for (int i = 1; i <= n; i++) fact[i] = fact[i - 1] * i % MOD;
    inv_fact[n] = power(fact[n], MOD - 2);
    for (int i = n; i > 0; i--) inv_fact[i - 1] = inv_fact[i] * i % MOD;
}
long long nCr(int n, int r) {
    if (r < 0 || r > n) return 0;
    return fact[n] * inv_fact[r] % MOD * inv_fact[n - r] % MOD;
}
