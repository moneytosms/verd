// coordinate compression: replaces each value of a (n numbers) by its rank among the distinct values
#include <stdlib.h>
static int cmp_ll(const void *x, const void *y) {
    long long a = *(const long long *)x, b = *(const long long *)y;
    return (a > b) - (a < b);
}
void compress(long long *a, int n) {
    long long *v = malloc(n * sizeof *v);
    int k = 0;
    for (int i = 0; i < n; i++) v[i] = a[i];
    qsort(v, n, sizeof *v, cmp_ll);
    for (int i = 0; i < n; i++)
        if (i == 0 || v[i] != v[i - 1]) v[k++] = v[i];
    for (int i = 0; i < n; i++) {
        int l = 0, r = k - 1;
        while (l < r) {
            int mid = (l + r) / 2;
            if (v[mid] < a[i]) l = mid + 1;
            else r = mid;
        }
        a[i] = l;
    }
    free(v);
}
