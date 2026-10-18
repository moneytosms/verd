// KMP prefix function: pi[i] is the length of the longest proper prefix of s[0..i] that is also a suffix
void prefix_function(const char *s, int n, int *pi) {
    if (n > 0) pi[0] = 0;
    for (int i = 1; i < n; i++) {
        int k = pi[i - 1];
        while (k > 0 && s[i] != s[k]) k = pi[k - 1];
        if (s[i] == s[k]) k++;
        pi[i] = k;
    }
}
