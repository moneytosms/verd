# KMP prefix function: pi[i] is the length of the longest proper prefix of s[:i+1] that is also a suffix
def prefix_function(s):
    pi = [0] * len(s)
    for i in range(1, len(s)):
        k = pi[i - 1]
        while k > 0 and s[i] != s[k]:
            k = pi[k - 1]
        if s[i] == s[k]:
            k += 1
        pi[i] = k
    return pi
