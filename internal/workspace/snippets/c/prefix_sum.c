// prefix sums of a (n numbers): p[i] = a[0] + ... + a[i-1]; sum of a[l..r) is p[r] - p[l]
static long long p[200006];
for (int i = 0; i < n; i++) p[i + 1] = p[i] + a[i];
