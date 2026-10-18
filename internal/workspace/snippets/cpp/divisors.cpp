// all divisors of n in increasing order, O(sqrt n)
vector<long long> divisors(long long n) {
    vector<long long> small, large;
    for (long long d = 1; d * d <= n; d++) {
        if (n % d) continue;
        small.push_back(d);
        if (d != n / d) large.push_back(n / d);
    }
    small.insert(small.end(), large.rbegin(), large.rend());
    return small;
}
