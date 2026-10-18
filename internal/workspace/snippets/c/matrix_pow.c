// square matrix multiply and power modulo mod for k x k matrices (k <= MK): O(k^3 log e)
#define MK 50
void mat_mul(int k, long long a[][MK], long long b[][MK], long long c[][MK], long long mod) {
    static long long t[MK][MK];
    for (int i = 0; i < k; i++)
        for (int j = 0; j < k; j++) t[i][j] = 0;
    for (int i = 0; i < k; i++)
        for (int l = 0; l < k; l++)
            for (int j = 0; j < k; j++) t[i][j] = (t[i][j] + a[i][l] * b[l][j]) % mod;
    for (int i = 0; i < k; i++)
        for (int j = 0; j < k; j++) c[i][j] = t[i][j];
}
void mat_pow(int k, long long a[][MK], long long e, long long mod, long long r[][MK]) {
    for (int i = 0; i < k; i++)
        for (int j = 0; j < k; j++) r[i][j] = i == j;
    for (; e > 0; e >>= 1) {
        if (e & 1) mat_mul(k, r, a, r, mod);
        mat_mul(k, a, a, a, mod);
    }
}
