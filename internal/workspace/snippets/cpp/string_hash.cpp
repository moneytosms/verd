// polynomial string hash modulo 2^61 - 1 with a random base; get(l, r) hashes s[l..r) in O(1)
struct Hash {
    static const unsigned long long M = (1ULL << 61) - 1;
    vector<unsigned long long> h, pw;
    static unsigned long long mul(unsigned long long a, unsigned long long b) {
        __uint128_t c = (__uint128_t)a * b;
        unsigned long long r = (unsigned long long)(c & M) + (unsigned long long)(c >> 61);
        r = (r & M) + (r >> 61);
        return r == M ? 0 : r;
    }
    Hash(const string &s) : h(s.size() + 1), pw(s.size() + 1, 1) {
        static const unsigned long long B = chrono::steady_clock::now().time_since_epoch().count() % (M - 1000) + 500;
        for (size_t i = 0; i < s.size(); i++) {
            pw[i + 1] = mul(pw[i], B);
            h[i + 1] = (mul(h[i], B) + s[i]) % M;
        }
    }
    unsigned long long get(int l, int r) {
        unsigned long long v = h[r] + M - mul(h[l], pw[r - l]);
        return v >= M ? v - M : v;
    }
};
