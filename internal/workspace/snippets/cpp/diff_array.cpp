// difference array: range_add adds v to a[l..r) in O(1); diff_apply prefix-sums d into the final values
void range_add(vector<long long> &d, int l, int r, long long v) {
    d[l] += v;
    d[r] -= v;
}
void diff_apply(vector<long long> &d) {
    for (size_t i = 1; i < d.size(); i++) d[i] += d[i - 1];
}
