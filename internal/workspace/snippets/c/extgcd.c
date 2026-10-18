// extended gcd: returns g = gcd(a, b) and sets *x, *y with a*x + b*y = g
long long extgcd(long long a, long long b, long long *x, long long *y) {
    if (!b) {
        *x = 1, *y = 0;
        return a;
    }
    long long g = extgcd(b, a % b, y, x);
    *y -= a / b * *x;
    return g;
}
