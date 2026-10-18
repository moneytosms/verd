// square matrix multiply and power modulo mod: O(k^3 log e)
using Mat = vector<vector<long long>>;
Mat mat_mul(const Mat &a, const Mat &b, long long mod) {
    int k = a.size();
    Mat c(k, vector<long long>(k));
    for (int i = 0; i < k; i++)
        for (int l = 0; l < k; l++)
            for (int j = 0; j < k; j++) c[i][j] = (c[i][j] + a[i][l] * b[l][j]) % mod;
    return c;
}
Mat mat_pow(Mat a, long long e, long long mod) {
    int k = a.size();
    Mat r(k, vector<long long>(k));
    for (int i = 0; i < k; i++) r[i][i] = 1;
    for (; e > 0; e >>= 1, a = mat_mul(a, a, mod))
        if (e & 1) r = mat_mul(r, a, mod);
    return r;
}
