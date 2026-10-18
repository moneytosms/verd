// KMP prefix function: pi[i] is the length of the longest proper prefix of s[0..i] that is also a suffix
vector<int> prefix_function(const string &s) {
    vector<int> pi(s.size());
    for (size_t i = 1; i < s.size(); i++) {
        int k = pi[i - 1];
        while (k > 0 && s[i] != s[k]) k = pi[k - 1];
        if (s[i] == s[k]) k++;
        pi[i] = k;
    }
    return pi;
}
