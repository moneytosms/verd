// modular exponentiation: b^e mod m in O(log e); b * b must fit in long long (m < 3e9)
long long modpow(long long b, long long e, long long m) {
    long long r = 1 % m;
    for (b %= m; e > 0; e >>= 1, b = b * b % m)
        if (e & 1) r = r * b % m;
    return r;
}
