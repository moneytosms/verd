// Z function: z[i] is the length of the longest common prefix of s and s[i..]
void z_function(const char *s, int n, int *z) {
    if (n > 0) z[0] = 0;
    for (int i = 1, l = 0, r = 0; i < n; i++) {
        z[i] = 0;
        if (i < r) z[i] = z[i - l] < r - i ? z[i - l] : r - i;
        while (i + z[i] < n && s[z[i]] == s[i + z[i]]) z[i]++;
        if (i + z[i] > r) l = i, r = i + z[i];
    }
}
