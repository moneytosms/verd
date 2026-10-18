# smallest-prime-factor sieve: build_spf(n) once, then factorize(x, spf) lists the prime factors of x <= n in O(log x)
def build_spf(n):
    spf = list(range(n + 1))
    for i in range(2, int(n**0.5) + 1):
        if spf[i] == i:
            for j in range(i * i, n + 1, i):
                if spf[j] == j:
                    spf[j] = i
    return spf


def factorize(x, spf):
    f = []
    while x > 1:
        f.append(spf[x])
        x //= spf[x]
    return f
