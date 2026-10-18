// n choose r modulo a prime: build_fact(n) in O(n), then nCr(n, r) in O(1)
const long long MOD = 1e9 + 7;
vector<long long> fact, inv_fact;
long long power(long long b, long long e) {
    long long r = 1;
    for (b %= MOD; e > 0; e >>= 1, b = b * b % MOD)
        if (e & 1) r = r * b % MOD;
    return r;
}
void build_fact(int n) {
    fact.assign(n + 1, 1);
    inv_fact.assign(n + 1, 1);
    for (int i = 1; i <= n; i++) fact[i] = fact[i - 1] * i % MOD;
    inv_fact[n] = power(fact[n], MOD - 2);
    for (int i = n; i > 0; i--) inv_fact[i - 1] = inv_fact[i] * i % MOD;
}
long long nCr(int n, int r) {
    if (r < 0 || r > n) return 0;
    return fact[n] * inv_fact[r] % MOD * inv_fact[n - r] % MOD;
}
