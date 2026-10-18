// prefix sums of a: p[i] = a[0] + ... + a[i-1]; sum of a[l..r) is p[r] - p[l]
vector<long long> p(a.size() + 1);
for (size_t i = 0; i < a.size(); i++) p[i + 1] = p[i] + a[i];
