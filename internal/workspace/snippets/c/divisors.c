// all divisors of n in increasing order into out; returns how many, O(sqrt n)
int divisors(long long n, long long *out) {
    int k = 0, lo = 0;
    long long d;
    for (d = 1; d * d <= n; d++)
        if (n % d == 0) out[k++] = d;
    lo = k;
    for (int i = lo - 1; i >= 0; i--)
        if (out[i] != n / out[i]) out[k++] = n / out[i];
    return k;
}
