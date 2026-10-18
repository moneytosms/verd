// coordinate compression: replaces each value of a by its rank among the distinct values
void compress(vector<long long> &a) {
    vector<long long> v(a);
    sort(v.begin(), v.end());
    v.erase(unique(v.begin(), v.end()), v.end());
    for (auto &x : a) x = lower_bound(v.begin(), v.end(), x) - v.begin();
}
