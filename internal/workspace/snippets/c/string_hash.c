// polynomial string hash modulo 2^61 - 1 (gcc __int128): hash_build(s, n) once, then hash_get(l, r) hashes s[l..r) in O(1)
#include <stdlib.h>
#include <time.h>
typedef unsigned long long u64;
#define HM ((1ULL << 61) - 1)
#define HASH_N 1000006
u64 hash_h[HASH_N], hash_pw[HASH_N];
static u64 hash_mul(u64 a, u64 b) {
    unsigned __int128 c = (unsigned __int128)a * b;
    u64 r = (u64)(c & HM) + (u64)(c >> 61);
    r = (r & HM) + (r >> 61);
    return r == HM ? 0 : r;
}
void hash_build(const char *s, int n) {
    srand((unsigned)time(NULL));
    u64 base = ((u64)rand() << 20 ^ (u64)rand()) % (HM - 1000) + 500;
    hash_pw[0] = 1;
    hash_h[0] = 0;
    for (int i = 0; i < n; i++) {
        hash_pw[i + 1] = hash_mul(hash_pw[i], base);
        hash_h[i + 1] = (hash_mul(hash_h[i], base) + (unsigned char)s[i]) % HM;
    }
}
u64 hash_get(int l, int r) {
    u64 v = hash_h[r] + HM - hash_mul(hash_h[l], hash_pw[r - l]);
    return v >= HM ? v - HM : v;
}
