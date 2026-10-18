// smallest-prime-factor sieve: build_spf(n) once, then factorize(x) lists the prime factors of x <= n in O(log x)
vector<int> spf;
void build_spf(int n) {
    spf.assign(n + 1, 0);
    for (int i = 2; i <= n; i++)
        if (!spf[i])
            for (int j = i; j <= n; j += i)
                if (!spf[j]) spf[j] = i;
}
vector<int> factorize(int x) {
    vector<int> f;
    for (; x > 1; x /= spf[x]) f.push_back(spf[x]);
    return f;
}
