// sparse table for range minimum on a static array: sparse_build in O(n log n), sparse_query of [l, r) in O(1)
#define SP_N 200005
#define SP_LOG 18
int sp[SP_LOG][SP_N];
void sparse_build(const int *a, int n) {
    for (int i = 0; i < n; i++) sp[0][i] = a[i];
    for (int j = 1; (1 << j) <= n; j++)
        for (int i = 0; i + (1 << j) <= n; i++) {
            int x = sp[j - 1][i], y = sp[j - 1][i + (1 << (j - 1))];
            sp[j][i] = x < y ? x : y;
        }
}
int sparse_query(int l, int r) {
    int j = 31 - __builtin_clz(r - l);
    int x = sp[j][l], y = sp[j][r - (1 << j)];
    return x < y ? x : y;
}
