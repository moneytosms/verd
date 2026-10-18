// length of the longest strictly increasing subsequence in O(n log n)
int lis(const vector<int> &a) {
    vector<int> t;
    for (int x : a) {
        auto it = lower_bound(t.begin(), t.end(), x);
        if (it == t.end()) t.push_back(x);
        else *it = x;
    }
    return t.size();
}
